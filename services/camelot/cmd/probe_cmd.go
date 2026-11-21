package cmd

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/Wigata-Intech/w-tools/cli"

	"github.com/mrdhira/camelot/services/camelot/config"
)

// probeCommand builds "probe": GET /healthz on the listen port over
// loopback; exit 1 unless 200. The container health check (scratch has no shell).
func probeCommand(load func() (*config.Config, error)) *cli.Command {
	return &cli.Command{
		Name:  "probe",
		Short: "health check of a running serve on this host (exit 1 unless healthy)",
		Run: func(ctx context.Context, _ []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			_, port, err := net.SplitHostPort(cfg.Listen)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+port+"/healthz", nil)
			if err != nil {
				return err
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			defer func() { _ = resp.Body.Close() }()
			_, _ = io.Copy(io.Discard, resp.Body)
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("probe: %s", resp.Status)
			}
			return nil
		},
	}
}
