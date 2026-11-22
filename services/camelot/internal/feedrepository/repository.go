// Package feedrepository stores when the feed was last marked seen.
package feedrepository

import (
	"context"
	"database/sql"
	"embed"

	"github.com/mrdhira/camelot/services/camelot/internal/feedmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/feedrepository/sqlite"
	"github.com/mrdhira/camelot/services/camelot/internal/pkg/sqlitedb"
)

//go:embed sqlite/migrations/*.sql
var migrations embed.FS

// Migrations is the repository's migration set.
var Migrations = sqlitedb.MigrationSet{Name: "feed", FS: migrations, Dir: "sqlite/migrations"}

// Repository is the seen marker.
type Repository interface {
	// SeenSelect returns the marker; nil when the feed was never marked seen.
	SeenSelect(ctx context.Context) (*feedmodel.Seen, error)
	// SeenUpdate records the marker, never moving either event ID back.
	SeenUpdate(ctx context.Context, seen *feedmodel.Seen) error
}

// NewSQLite returns the Repository on db.
func NewSQLite(db *sql.DB) Repository {
	return sqlite.New(db)
}
