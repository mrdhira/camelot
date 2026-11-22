package cmd

import (
	"context"
	"io"
	"time"

	"github.com/Wigata-Intech/w-tools/cli"
	"github.com/axadrn/shadcn-templ/v2/utils"

	"github.com/mrdhira/camelot/services/camelot/config"
	"github.com/mrdhira/camelot/services/camelot/internal/cockpithandler"
	"github.com/mrdhira/camelot/services/camelot/internal/searchservice"
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
			cockpithandler.New(cockpithandler.Config{DocRoots: a.config.DocRoots}, a.logger, a.tasks, a.docs, a.insights, a.feed, searchservice.New(a.tasks, a.docs)).Routes(srv)
			go syncDocs(ctx, a)
			a.logger.InfoContext(ctx, "camelot serving", "url", "http://"+a.config.Listen, "vault", a.config.Vault)
			return srv.Run(ctx)
		}),
	}
}

// appZone sets the zone the pages show times in.
func appZone(loc *time.Location) {
	app.Zone = loc
}

// docSyncEvery is how often serve compares the document files with their snapshots.
const docSyncEvery = 30 * time.Second

// syncDocs records document changes every docSyncEvery until ctx ends, so an
// edit is recorded near the time it was made even with no page open.
func syncDocs(ctx context.Context, a *deps) {
	t := time.NewTicker(docSyncEvery)
	defer t.Stop()
	for {
		if err := a.docs.Sync(ctx); err != nil && ctx.Err() == nil {
			a.logger.WarnContext(ctx, "document sync failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
