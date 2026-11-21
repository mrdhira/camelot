// Package config declares and validates Camelot's settings: flags, each
// also read from CAMELOT_<NAME> in the environment.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Wigata-Intech/w-tools/cli"
)

// DefaultListen is the web server's default address: loopback only.
const DefaultListen = "127.0.0.1:7700"

// DefaultDocRoots are the document roots, relative to the vault.
var DefaultDocRoots = []string{"wiki/wigata-intech", "workspaces/wigata-monolithic/docs/trd"}

// Config is the validated settings.
type Config struct {
	Vault    string
	DB       string
	DocRoots []string
	Listen   string
	// Hosts are the Host header values the web server answers; any other gets 421.
	Hosts       []string
	BehindProxy bool
	Actor       string
	Zone        *time.Location
	LogLevel    slog.Level
}

// Loader holds the declared flags until Load validates them.
type Loader struct {
	vault    *string
	db       *string
	docRoots *[]string
	listen   *string
	hosts    *[]string
	proxy    *bool
	actor    *string
	zone     *string
	logLevel *string
}

// Declare declares the flags on fs.
func Declare(fs *cli.FlagSet) *Loader {
	return &Loader{
		vault:    fs.String("vault", "", "the vault root (dita-os); default: the nearest parent of the working directory holding tasks/TASK.md"),
		db:       fs.String("db", "", "the SQLite database; default <vault>/.camelot/camelot.db"),
		docRoots: fs.StringList("doc-roots", DefaultDocRoots, "document roots, relative to the vault"),
		listen:   fs.String("listen", DefaultListen, "the web server's address; loopback only unless --behind-proxy"),
		hosts:    fs.StringList("hosts", nil, "extra Host header values the web server answers (camelot.localhost behind Caddy)"),
		proxy:    fs.Bool("behind-proxy", false, "in a container behind the local reverse proxy: any listen address, and --hosts required"),
		actor:    fs.String("actor", "dita", "who makes the changes: an agent's name, or dhira"),
		zone:     fs.String("time-zone", "Asia/Tokyo", "the IANA time zone of pages and of document dates"),
		logLevel: fs.String("log-level", "info", "debug, info, warn or error"),
	}
}

// Load validates the flags into a Config.
func (l *Loader) Load() (*Config, error) {
	c := &Config{DocRoots: *l.docRoots, Listen: *l.listen, Actor: strings.TrimSpace(*l.actor)}
	vault := *l.vault
	if vault == "" {
		found, err := findVault()
		if err != nil {
			return nil, err
		}
		vault = found
	}
	abs, err := filepath.Abs(vault)
	if err != nil {
		return nil, fmt.Errorf("invalid configuration: -vault: %w", err)
	}
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("invalid configuration: -vault (env CAMELOT_VAULT): %s is not a directory", abs)
	}
	c.Vault = abs
	c.DB = *l.db
	if c.DB == "" {
		c.DB = filepath.Join(abs, ".camelot", "camelot.db")
	}
	for _, root := range c.DocRoots {
		if filepath.IsAbs(root) || strings.Contains(filepath.ToSlash(filepath.Clean(root)), "..") {
			return nil, fmt.Errorf("invalid configuration: -doc-roots (env CAMELOT_DOC_ROOTS): %q must be relative and inside the vault", root)
		}
	}
	host, port, err := net.SplitHostPort(c.Listen)
	if err != nil {
		return nil, fmt.Errorf("invalid configuration: -listen (env CAMELOT_LISTEN): %w", err)
	}
	c.BehindProxy = *l.proxy
	if ip := net.ParseIP(host); !c.BehindProxy && host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return nil, fmt.Errorf("invalid configuration: -listen (env CAMELOT_LISTEN): %q must be a loopback address; Camelot has no sign-in", c.Listen)
	}
	if c.BehindProxy && len(*l.hosts) == 0 {
		return nil, errors.New("invalid configuration: -behind-proxy (env CAMELOT_BEHIND_PROXY) needs -hosts (env CAMELOT_HOSTS), the names the proxy serves")
	}
	c.Hosts = []string{"127.0.0.1:" + port, "localhost:" + port, "[::1]:" + port}
	for _, h := range *l.hosts {
		h = strings.ToLower(strings.TrimSpace(h))
		if h == "" || strings.ContainsAny(h, "/ ") {
			return nil, fmt.Errorf("invalid configuration: -hosts (env CAMELOT_HOSTS): %q is not a host name", h)
		}
		c.Hosts = append(c.Hosts, h)
	}
	if c.Actor == "" || strings.ContainsAny(c.Actor, "\r\n") {
		return nil, errors.New("invalid configuration: -actor (env CAMELOT_ACTOR): must be one non-empty line")
	}
	loc, err := time.LoadLocation(*l.zone)
	if err != nil {
		return nil, fmt.Errorf("invalid configuration: -time-zone (env CAMELOT_TIME_ZONE): %w", err)
	}
	c.Zone = loc
	if err := c.LogLevel.UnmarshalText([]byte(*l.logLevel)); err != nil {
		return nil, fmt.Errorf("invalid configuration: -log-level (env CAMELOT_LOG_LEVEL): %w", err)
	}
	return c, nil
}

// findVault walks up from the working directory to the first directory holding tasks/TASK.md.
func findVault() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("invalid configuration: -vault: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "tasks", "TASK.md")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("invalid configuration: -vault (env CAMELOT_VAULT) is not set and no parent directory holds tasks/TASK.md")
		}
		dir = parent
	}
}
