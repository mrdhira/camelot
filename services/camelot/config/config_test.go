package config_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wigata-Intech/w-tools/cli"

	"github.com/mrdhira/camelot/services/camelot/config"
)

func TestLoad(t *testing.T) {
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "tasks"), 0o700); err != nil {
		t.Fatal(err)
	}
	type expected struct {
		listen string
		db     string
		zone   string
		hosts  string
		err    string
	}
	tests := []struct {
		name     string
		input    []string
		expected expected
	}{
		{name: "defaults on the given vault", input: nil, expected: expected{listen: "127.0.0.1:7700", db: filepath.Join(vault, ".camelot", "camelot.db"), zone: "Asia/Tokyo"}},
		{name: "localhost and IPv6 loopback", input: []string{"-listen", "[::1]:9000", "-time-zone", "UTC"}, expected: expected{listen: "[::1]:9000", db: filepath.Join(vault, ".camelot", "camelot.db"), zone: "UTC"}},
		{name: "every interface is refused: no sign-in", input: []string{"-listen", ":7700"}, expected: expected{err: "must be a loopback address"}},
		{name: "behind the proxy any address, with its host names", input: []string{"-listen", "0.0.0.0:7700", "-behind-proxy", "-hosts", "Camelot.localhost"}, expected: expected{listen: "0.0.0.0:7700", db: filepath.Join(vault, ".camelot", "camelot.db"), zone: "Asia/Tokyo", hosts: "127.0.0.1:7700 localhost:7700 [::1]:7700 camelot.localhost"}},
		{name: "behind the proxy without host names", input: []string{"-listen", "0.0.0.0:7700", "-behind-proxy"}, expected: expected{err: "needs -hosts"}},
		{name: "a LAN address is refused", input: []string{"-listen", "192.168.1.10:7700"}, expected: expected{err: "must be a loopback address"}},
		{name: "a document root outside the vault", input: []string{"-doc-roots", "../elsewhere"}, expected: expected{err: "inside the vault"}},
		{name: "an empty actor", input: []string{"-actor", " "}, expected: expected{err: "-actor"}},
		{name: "an unknown time zone", input: []string{"-time-zone", "Mars/Olympus"}, expected: expected{err: "-time-zone"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var loader *config.Loader
			var got *config.Config
			var loadErr error
			cmd := &cli.Command{
				Name:  "camelot",
				Flags: func(fs *cli.FlagSet) { loader = config.Declare(fs) },
				Commands: []*cli.Command{{
					Name: "run",
					Run: func(_ context.Context, _ []string) error {
						got, loadErr = loader.Load()
						return nil
					},
				}},
			}
			os.Args = append([]string{"camelot", "-vault", vault}, append(tt.input, "run")...)
			if code := cmd.Execute(t.Context()); code != 0 {
				t.Fatalf("Execute() = %d", code)
			}
			if tt.expected.err != "" {
				if loadErr == nil || !strings.Contains(loadErr.Error(), tt.expected.err) {
					t.Fatalf("Load() error = %v, want it to mention %q", loadErr, tt.expected.err)
				}
				return
			}
			if loadErr != nil {
				t.Fatalf("Load() error = %v", loadErr)
			}
			if tt.expected.hosts != "" && strings.Join(got.Hosts, " ") != tt.expected.hosts {
				t.Errorf("Hosts = %v, want %s", got.Hosts, tt.expected.hosts)
			}
			if got.Listen != tt.expected.listen || got.DB != tt.expected.db || got.Zone.String() != tt.expected.zone {
				t.Errorf("Load() = %s %s %s, want %s %s %s", got.Listen, got.DB, got.Zone, tt.expected.listen, tt.expected.db, tt.expected.zone)
			}
		})
	}
}
