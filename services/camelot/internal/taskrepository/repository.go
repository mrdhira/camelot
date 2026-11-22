// Package taskrepository stores cards, their links and their history.
package taskrepository

import (
	"context"
	"database/sql"
	"embed"
	"time"

	"github.com/mrdhira/camelot/services/camelot/internal/pkg/sqlitedb"
	"github.com/mrdhira/camelot/services/camelot/internal/taskmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/taskrepository/folder"
	"github.com/mrdhira/camelot/services/camelot/internal/taskrepository/sqlite"
)

//go:embed sqlite/migrations/*.sql
var migrations embed.FS

// Migrations is the repository's migration set.
var Migrations = sqlitedb.MigrationSet{Name: "task", FS: migrations, Dir: "sqlite/migrations"}

// Repository is the card store. A nil card means nothing matched.
type Repository interface {
	// CardInsert stores a new card with its links and events in one
	// transaction; taskmodel.ErrExists when the ID was ever used.
	CardInsert(ctx context.Context, card *taskmodel.Card, events []*taskmodel.Event) error
	// CardSelectByID returns the card unless it is missing or deleted.
	CardSelectByID(ctx context.Context, id string) (*taskmodel.Card, error)
	// CardSelect returns the live cards matching filter, ordered by ID.
	CardSelect(ctx context.Context, filter *taskmodel.CardFilter) ([]*taskmodel.Card, error)
	// CardUpdate replaces the card's fields and links and appends events in
	// one transaction when its stored updated_at equals readAt; nil when
	// it changed meanwhile or is gone; taskmodel.ErrInvalidLink when the
	// links, as committed, would make a dependency cycle.
	CardUpdate(ctx context.Context, card *taskmodel.Card, readAt time.Time, events []*taskmodel.Event) (*taskmodel.Card, error)
	// CardDelete marks the card deleted and appends event; false when it
	// was missing or already deleted; taskmodel.ErrInvalidLink while any
	// card depends on it or belongs to it.
	CardDelete(ctx context.Context, id string, at time.Time, event *taskmodel.Event) (bool, error)
	// CardMaxNumber returns the highest N of IDs "<prefix>-N", deleted cards included; 0 when none.
	CardMaxNumber(ctx context.Context, prefix string) (int, error)
	// CardEventMaxID returns the highest card event ID; 0 when none.
	CardEventMaxID(ctx context.Context) (int64, error)
	// CardEventSelect returns events matching filter, newest first.
	CardEventSelect(ctx context.Context, filter *taskmodel.EventFilter) ([]*taskmodel.Event, error)
}

// Folders reads card folders.
type Folders interface {
	// FolderFiles lists the Markdown files directly in folder; taskmodel.ErrFolderMissing when it does not exist.
	FolderFiles(ctx context.Context, folder string) ([]string, error)
	// FolderFileRead returns one Markdown file of folder; taskmodel.ErrFileNotFound when absent or not readable.
	FolderFileRead(ctx context.Context, folder, name string) ([]byte, error)
}

// NewFolders returns the Folders on the vault.
func NewFolders(vault string) Folders {
	return folder.New(vault)
}

// NewSQLite returns the Repository on db.
func NewSQLite(db *sql.DB) Repository {
	return sqlite.New(db)
}
