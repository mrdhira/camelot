package cmd

import (
	"context"
	"io"
	"time"

	"github.com/Wigata-Intech/w-tools/cli"
	"github.com/axadrn/shadcn-templ/v2/utils"

	"github.com/mrdhira/camelot/services/camelot/config"
	"github.com/mrdhira/camelot/services/camelot/internal/cockpithandler"
	"github.com/mrdhira/camelot/services/camelot/src/app"
)

// serveCommand builds "serve": the web UI on the loopback listener until ctx ends.
func serveCommand(load func() (*config.Config, error), logOut io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "serve",
		Short: "serve the cockpit web UI (loopback only)",
		Run: runWith(load, logOut, func(ctx context.Context, a *deps) error {
			appZone(a.config.Zone)
			if a.config.BehindProxy {
				release, err := claimOwner(a.config.DB)
				if err != nil {
					return err
				}
				defer release()
			}
			// utils.CN initializes tailwind-merge-go unsynchronized on first use.
			utils.CN("")
			srv := cockpithandler.NewServer(a.config.Listen, a.config.Hosts, a.logger)
			cockpithandler.New(cockpithandler.Config{DocRoots: a.config.DocRoots}, a.logger, a.tasks, a.docs, a.insights).Routes(srv)
			a.logger.InfoContext(ctx, "camelot serving", "url", "http://"+a.config.Listen, "vault", a.config.Vault)
			return srv.Run(ctx)
		}),
	}
}

// appZone sets the zone the pages show times in.
func appZone(loc *time.Location) {
	app.Zone = loc
}
