package cmd

import (
	"context"
	"io"
	"os"

	"github.com/Wigata-Intech/w-tools/cli"

	"github.com/mrdhira/camelot/services/camelot/config"
	"github.com/mrdhira/camelot/services/camelot/internal/mcphandler"
)

// Version is the binary's version, stamped at build time.
var Version = "dev"

// mcpCommand builds "mcp": the MCP server on stdin and stdout; logs go to logOut (stderr).
func mcpCommand(load func() (*config.Config, error), out, logOut io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "mcp",
		Short: "serve the MCP tools on stdio for agents (JSON-RPC, one message per line)",
		Run: runWith(load, logOut, func(ctx context.Context, a *deps) error {
			guard := func() error { return checkOwner(a.config.DB) }
			server := mcphandler.New(mcphandler.Config{Name: "camelot", Version: Version, Actor: a.config.Actor, Guard: guard}, a.logger, a.tasks, a.docs, a.insights)
			return server.Serve(ctx, os.Stdin, out)
		}),
	}
}
