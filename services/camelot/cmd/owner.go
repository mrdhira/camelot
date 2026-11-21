package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// ownerFile records which host serves the database from a container.
// SQLite's locks and WAL memory work only between processes of one kernel:
// while the container serves, a process on any other host (the Mac's CLI or
// MCP) must not open the file.
const ownerFile = "server.lock"

// owner is the content of the owner file.
type owner struct {
	Host  string    `json:"host"`
	Since time.Time `json:"since"`
}

// ownerPath is the owner file beside the database.
func ownerPath(db string) string {
	return filepath.Join(filepath.Dir(db), ownerFile)
}

// checkOwner refuses when another host owns the database.
func checkOwner(db string) error {
	content, err := os.ReadFile(ownerPath(db)) //nolint:gosec // beside the configured database
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", ownerFile, err)
	}
	var o owner
	if err := json.Unmarshal(content, &o); err != nil {
		return fmt.Errorf("read %s: %w", ownerFile, err)
	}
	host, err := os.Hostname()
	if err != nil {
		return err
	}
	if o.Host != host {
		return fmt.Errorf("the database is served by the Camelot container (host %q since %s): run through workspaces/camelot/scripts/camelot, "+
			"which uses docker exec, or stop the stack with make down (a crashed stack leaves %s; make down removes it)",
			o.Host, o.Since.Format(time.RFC3339), ownerPath(db))
	}
	return nil
}

// claimOwner records this host as the database's owner; release removes the record.
func claimOwner(db string) (release func(), err error) {
	host, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	content, err := json.Marshal(owner{Host: host, Since: time.Now().UTC()})
	if err != nil {
		return nil, err
	}
	path := ownerPath(db)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return nil, fmt.Errorf("write %s: %w", ownerFile, err)
	}
	return func() { _ = os.Remove(path) }, nil
}
