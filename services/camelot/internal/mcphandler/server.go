// Package mcphandler serves Camelot's operations as MCP tools over stdio:
// JSON-RPC 2.0, one message per line. It answers both protocol eras: the
// initialize handshake (2025-11-25 and earlier) and server/discover with
// the version in each request's _meta (2026-07-28).
package mcphandler

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"sync"

	"github.com/mrdhira/camelot/services/camelot/internal/docservice"
	"github.com/mrdhira/camelot/services/camelot/internal/insightservice"
	"github.com/mrdhira/camelot/services/camelot/internal/taskservice"
)

// SupportedVersions are the protocol revisions served, newest first.
var SupportedVersions = []string{"2026-07-28", "2025-11-25", "2025-06-18", "2025-03-26"}

// legacyDefault is answered to an initialize naming an unknown version.
const legacyDefault = "2025-11-25"

// versionKey is where a 2026-07-28 request names its protocol version.
const versionKey = "io.modelcontextprotocol/protocolVersion"

// maxMessage bounds one incoming line.
const maxMessage = 8 << 20

// JSON-RPC and MCP error codes.
const (
	codeParse              = -32700
	codeInvalidRequest     = -32600
	codeMethodNotFound     = -32601
	codeInvalidParams      = -32602
	codeUnsupportedVersion = -32022
)

// instructions tell an agent how to use the tools.
const instructions = "Camelot is Dhira's cockpit of cards and documents. Record your work as it happens: " +
	"create a card before non-trivial work (prefix DITA, WIG…), move it as the dev loop advances " +
	"(plan → wip → qa → review; done only after Dhira's commit), set waiting with a one-line question when a decision is Dhira's, " +
	"and pass your own name as actor. Dhira reads the web UI; facts only, every number with its source."

// Config is the server's settings.
type Config struct {
	// Name and Version are reported as serverInfo.
	Name    string
	Version string
	// Actor is recorded on changes whose call names none.
	Actor string
	// Guard, when set, runs before every tool call; its error refuses the
	// call (the database was taken over by a process on another host).
	Guard func() error
}

// Server is the MCP server.
type Server struct {
	config      Config
	logger      *slog.Logger
	tools       []tool
	initialized bool
	writeMu     sync.Mutex
}

// New returns the server over the services.
func New(config Config, logger *slog.Logger, taskSvc taskservice.Service, docSvc docservice.Service, insightSvc insightservice.Service) *Server {
	s := &Server{config: config, logger: logger}
	s.tools = newTools(config, taskSvc, docSvc, insightSvc)
	return s
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Serve reads requests from in and writes responses to out until in ends or ctx is done.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), maxMessage)
	for sc.Scan() {
		if ctx.Err() != nil {
			return nil
		}
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		if resp := s.handle(ctx, line); resp != nil {
			if err := s.write(out, resp); err != nil {
				return err
			}
		}
	}
	if err := sc.Err(); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("mcp: read: %w", err)
	}
	return nil
}

// write sends one response as one line.
func (s *Server) write(out io.Writer, resp *response) error {
	b, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("mcp: encode: %w", err)
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if _, err := out.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("mcp: write: %w", err)
	}
	return nil
}

// handle answers one message; nil for a notification.
func (s *Server) handle(ctx context.Context, line []byte) *response {
	var req request
	if err := json.Unmarshal(line, &req); err != nil {
		return &response{JSONRPC: "2.0", Error: &rpcError{Code: codeParse, Message: "parse error: " + err.Error()}}
	}
	if req.JSONRPC != "2.0" || req.Method == "" {
		return &response{JSONRPC: "2.0", ID: rawID(req.ID), Error: &rpcError{Code: codeInvalidRequest, Message: "invalid request"}}
	}
	if len(req.ID) == 0 {
		if req.Method == "notifications/initialized" {
			s.initialized = true
		}
		return nil
	}
	resp := &response{JSONRPC: "2.0", ID: rawID(req.ID)}
	result, rerr := s.dispatch(ctx, req)
	if rerr != nil {
		resp.Error = rerr
	} else {
		resp.Result = result
	}
	return resp
}

// dispatch runs one request.
func (s *Server) dispatch(ctx context.Context, req request) (any, *rpcError) {
	var params struct {
		ProtocolVersion string          `json:"protocolVersion"`
		Name            string          `json:"name"`
		Arguments       json.RawMessage `json:"arguments"`
		Meta            map[string]any  `json:"_meta"`
	}
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, &rpcError{Code: codeInvalidParams, Message: "invalid params: " + err.Error()}
		}
	}
	switch req.Method {
	case "initialize":
		version := params.ProtocolVersion
		if !slices.Contains(SupportedVersions, version) || version == SupportedVersions[0] {
			version = legacyDefault
		}
		s.initialized = true
		return map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      s.serverInfo(),
			"instructions":    instructions,
		}, nil
	case "ping":
		return map[string]any{}, nil
	}
	modern := false
	if v, ok := params.Meta[versionKey]; ok {
		version, _ := v.(string)
		if !slices.Contains(SupportedVersions, version) {
			return nil, &rpcError{Code: codeUnsupportedVersion, Message: "unsupported protocol version", Data: map[string]any{"supported": SupportedVersions, "requested": version}}
		}
		modern = true
	} else if !s.initialized && req.Method != "server/discover" {
		return nil, &rpcError{Code: codeInvalidParams, Message: "missing _meta." + versionKey + " (or initialize first)"}
	}
	switch req.Method {
	case "server/discover":
		return map[string]any{
			"resultType":        "complete",
			"supportedVersions": SupportedVersions,
			"capabilities":      map[string]any{"tools": map[string]any{"listChanged": false}},
			"instructions":      instructions,
			"ttlMs":             0,
			"cacheScope":        "public",
			"_meta":             map[string]any{"io.modelcontextprotocol/serverInfo": s.serverInfo()},
		}, nil
	case "tools/list":
		list := make([]map[string]any, 0, len(s.tools))
		for _, t := range s.tools {
			list = append(list, map[string]any{"name": t.name, "title": t.title, "description": t.description, "inputSchema": t.schema, "annotations": t.annotations})
		}
		return withResultType(map[string]any{"tools": list, "ttlMs": 0, "cacheScope": "public"}, modern), nil
	case "tools/call":
		if s.config.Guard != nil {
			if err := s.config.Guard(); err != nil {
				return withResultType(map[string]any{"content": []map[string]any{{"type": "text", "text": err.Error()}}, "isError": true}, modern), nil
			}
		}
		for _, t := range s.tools {
			if t.name == params.Name {
				return withResultType(s.call(ctx, t, params.Arguments), modern), nil
			}
		}
		return nil, &rpcError{Code: codeInvalidParams, Message: "unknown tool: " + params.Name}
	}
	return nil, &rpcError{Code: codeMethodNotFound, Message: "method not found: " + req.Method}
}

// call runs a tool: its result as JSON text and structured content, or its error with isError.
func (s *Server) call(ctx context.Context, t tool, args json.RawMessage) map[string]any {
	if len(args) == 0 {
		args = json.RawMessage("{}")
	}
	v, err := t.run(ctx, args)
	if err != nil {
		s.logger.InfoContext(ctx, "mcp tool refused", "tool", t.name, "error", err)
		return map[string]any{"content": []map[string]any{{"type": "text", "text": err.Error()}}, "isError": true}
	}
	text, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return map[string]any{"content": []map[string]any{{"type": "text", "text": err.Error()}}, "isError": true}
	}
	return map[string]any{"content": []map[string]any{{"type": "text", "text": string(text)}}, "structuredContent": v, "isError": false}
}

// serverInfo is the implementation's name and version.
func (s *Server) serverInfo() map[string]any {
	return map[string]any{"name": s.config.Name, "title": "Camelot", "version": s.config.Version}
}

// withResultType adds resultType for a 2026-07-28 request.
func withResultType(result map[string]any, modern bool) map[string]any {
	if modern {
		result["resultType"] = "complete"
	}
	return result
}

// rawID returns the request's ID as decoded JSON, or nil.
func rawID(id json.RawMessage) any {
	if len(id) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(id, &v); err != nil {
		return nil
	}
	return v
}
