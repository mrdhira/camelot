// Command camelot is Dita's cockpit: a local web UI for Dhira over cards,
// documents and insights, with a CLI and an MCP server for agents.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	_ "time/tzdata" // embedded zone data, so --time-zone works on any host

	"github.com/mrdhira/camelot/services/camelot/cmd"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := cmd.Root(os.Stdout, os.Stderr).Execute(ctx)
	stop()
	os.Exit(code)
}
