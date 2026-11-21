package mcphandler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/mrdhira/camelot/services/camelot/internal/insightservice"
	"github.com/mrdhira/camelot/services/camelot/internal/mcphandler"
	"github.com/mrdhira/camelot/services/camelot/internal/taskservice/taskservicetest"
)

const modernMeta = `"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}`

// serve sends lines to a fresh server and returns its responses by ID.
func serve(t *testing.T, lines ...string) map[float64]map[string]any {
	t.Helper()
	return serveGuarded(t, nil, lines...)
}

// serveGuarded is serve with a guard before every tool call.
func serveGuarded(t *testing.T, guard func() error, lines ...string) map[float64]map[string]any {
	t.Helper()
	h := taskservicetest.New(t)
	h.Seed(t)
	server := mcphandler.New(mcphandler.Config{Name: "camelot", Version: "test", Actor: "dita", Guard: guard}, slog.New(slog.NewTextHandler(io.Discard, nil)),
		h.Tasks, h.Docs, insightservice.New(h.Tasks, h.Docs))
	var out bytes.Buffer
	if err := server.Serve(context.Background(), strings.NewReader(strings.Join(lines, "\n")+"\n"), &out); err != nil {
		t.Fatal(err)
	}
	byID := map[float64]map[string]any{}
	for line := range strings.SplitSeq(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("response %q is not one JSON line: %v", line, err)
		}
		id, _ := m["id"].(float64)
		byID[id] = m
	}
	return byID
}

// at reads a nested field: at(m, "result", "tools").
func at(m map[string]any, path ...string) any {
	var v any = m
	for _, p := range path {
		obj, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = obj[p]
	}
	return v
}

func TestServe(t *testing.T) {
	type check struct {
		id    float64
		path  []string
		value any
	}
	tests := []struct {
		name     string
		input    []string
		expected []check
	}{
		{
			name: "2025-11-25: initialize, then list and call tools",
			input: []string{
				`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`,
				`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
				`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"create_task","arguments":{"prefix":"DITA","title":"From MCP","actor":"go-engineer"}}}`,
				`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"update_task","arguments":{"id":"DITA-002","status":"wip","note":"build"}}}`,
				`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"update_task","arguments":{"id":"DITA-002","status":"blocked"}}}`,
				`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"nope","arguments":{}}}`,
				`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"get_task","arguments":{"id":"DITA-002","extra":1}}}`,
			},
			expected: []check{
				{1, []string{"result", "protocolVersion"}, "2025-11-25"},
				{1, []string{"result", "serverInfo", "name"}, "camelot"},
				{2, []string{"result", "structuredContent", "id"}, "DITA-002"},
				{2, []string{"result", "isError"}, false},
				{3, []string{"result", "structuredContent", "status"}, "wip"},
				{4, []string{"result", "isError"}, true},
				{5, []string{"error", "code"}, float64(-32602)},
				{6, []string{"result", "isError"}, true},
			},
		},
		{
			name: "an unknown requested version gets one the server supports",
			input: []string{
				`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"1999-01-01"}}`,
			},
			expected: []check{{1, []string{"result", "protocolVersion"}, "2025-11-25"}},
		},
		{
			name: "2026-07-28: discover, then the version in each request's _meta",
			input: []string{
				`{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{` + modernMeta + `}}`,
				`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{` + modernMeta + `}}`,
				`{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2099-01-01"}}}`,
				`{"jsonrpc":"2.0","id":4,"method":"tools/list","params":{}}`,
				`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"insights","arguments":{},` + modernMeta + `}}`,
			},
			expected: []check{
				{1, []string{"result", "resultType"}, "complete"},
				{1, []string{"result", "cacheScope"}, "public"},
				{2, []string{"result", "resultType"}, "complete"},
				{3, []string{"error", "code"}, float64(-32022)},
				{4, []string{"error", "code"}, float64(-32602)},
				{5, []string{"result", "structuredContent", "cards"}, float64(1)},
			},
		},
		{
			name:     "malformed JSON and unknown methods",
			input:    []string{`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`, `{"jsonrpc":"2.0","id":2,"method":"resources/list","params":{}}`, `not json`},
			expected: []check{{2, []string{"error", "code"}, float64(-32601)}, {0, []string{"error", "code"}, float64(-32700)}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := serve(t, tt.input...)
			for _, c := range tt.expected {
				if v := at(got[c.id], c.path...); v != c.value {
					t.Errorf("id %v %v = %#v, want %#v (response %v)", c.id, c.path, v, c.value, got[c.id])
				}
			}
		})
	}
}

func TestToolsList(t *testing.T) {
	got := serve(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`, `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	tools, _ := at(got[2], "result", "tools").([]any)
	var names []string
	for _, tool := range tools {
		m := tool.(map[string]any)
		names = append(names, m["name"].(string))
		if at(m, "inputSchema", "type") != "object" {
			t.Errorf("%v inputSchema.type = %v, want object", m["name"], at(m, "inputSchema", "type"))
		}
	}
	want := "list_tasks get_task create_task update_task delete_task list_docs get_doc set_doc_status insights check"
	if strings.Join(names, " ") != want {
		t.Errorf("tools = %v, want %s", names, want)
	}
	if v := at(got[1], "result", "protocolVersion"); v != "2025-06-18" {
		t.Errorf("protocolVersion = %v, want the requested 2025-06-18", v)
	}
}

func TestGuardRefusesToolCalls(t *testing.T) {
	got := serveGuarded(t, func() error { return errors.New("the database is served by the Camelot container") },
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"create_task","arguments":{"prefix":"DITA","title":"x"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}`,
	)
	if at(got[2], "result", "isError") != true {
		t.Errorf("guarded call = %v, want isError", got[2])
	}
	if _, ok := at(got[3], "result", "tools").([]any); !ok {
		t.Errorf("tools/list = %v, want it answered (the guard covers tool calls only)", got[3])
	}
}
