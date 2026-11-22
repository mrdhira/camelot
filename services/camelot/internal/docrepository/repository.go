// Package docrepository reads and updates the document files.
package docrepository

import (
	"context"
	"database/sql"
	"embed"

	"github.com/mrdhira/camelot/services/camelot/internal/docmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/docrepository/markdown"
	"github.com/mrdhira/camelot/services/camelot/internal/docrepository/sqlite"
	"github.com/mrdhira/camelot/services/camelot/internal/pkg/sqlitedb"
)

// Repository is the document store.
type Repository interface {
	// DocSelect returns every document under the roots and the problems
	// found: a Markdown file whose frontmatter has a type is a document.
	DocSelect(ctx context.Context) ([]*docmodel.Doc, []docmodel.Problem, error)
	// DocRender returns the document at path (relative to the vault) with
	// the given frontmatter keys rewritten, every other byte kept; nothing is written.
	DocRender(ctx context.Context, path string, fields map[string]string) ([]byte, error)
	// DocWrite replaces the document at path with content.
	DocWrite(ctx context.Context, path string, content []byte) error
}

// History stores what Camelot last saw of each document and every change.
type History interface {
	// SnapshotSelect returns every stored snapshot.
	SnapshotSelect(ctx context.Context) ([]*docmodel.Snapshot, error)
	// SnapshotSync applies changes in one transaction, each only when the
	// stored snapshot still has its BaseHash, and records the baseline; it
	// returns the events written. write, when set, runs first inside the
	// transaction (a file write that must not race another process's sync).
	SnapshotSync(ctx context.Context, changes []*docmodel.Change, write func() error) ([]*docmodel.Event, error)
	// Baselined reports whether the first sync was recorded.
	Baselined(ctx context.Context) (bool, error)
	// DocEventMaxID returns the highest document event ID; 0 when none.
	DocEventMaxID(ctx context.Context) (int64, error)
	// DocEventSelect returns events matching filter, newest first.
	DocEventSelect(ctx context.Context, filter *docmodel.EventFilter) ([]*docmodel.Event, error)
}

//go:embed sqlite/migrations/*.sql
var migrations embed.FS

// Migrations is the history's migration set.
var Migrations = sqlitedb.MigrationSet{Name: "doc", FS: migrations, Dir: "sqlite/migrations"}

// NewSQLite returns the History on db.
func NewSQLite(db *sql.DB) History {
	return sqlite.New(db)
}

// NewMarkdown returns the Repository on the Markdown files under roots
// (relative to vault).
func NewMarkdown(vault string, roots []string) Repository {
	return markdown.New(vault, roots)
}
