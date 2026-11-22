// Package commentrepository stores comments and their resolutions.
package commentrepository

import (
	"context"
	"database/sql"
	"embed"

	"github.com/mrdhira/camelot/services/camelot/internal/commentmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/commentrepository/sqlite"
	"github.com/mrdhira/camelot/services/camelot/internal/pkg/sqlitedb"
)

//go:embed sqlite/migrations/*.sql
var migrations embed.FS

// Migrations is the repository's migration set.
var Migrations = sqlitedb.MigrationSet{Name: "comment", FS: migrations, Dir: "sqlite/migrations"}

// Repository is the comment store.
type Repository interface {
	// CommentInsert stores c and sets its ID; a second resolution of one comment is refused.
	CommentInsert(ctx context.Context, c *commentmodel.Comment) error
	// CommentGet returns one row; ErrNotFound when missing.
	CommentGet(ctx context.Context, id int64) (*commentmodel.Comment, error)
	// CommentSelect returns the rows matching the filter, oldest first.
	CommentSelect(ctx context.Context, filter *commentmodel.Filter) ([]*commentmodel.Comment, error)
	// CommentMaxID returns the highest row ID; 0 when none.
	CommentMaxID(ctx context.Context) (int64, error)
}

// NewSQLite returns the Repository on db.
func NewSQLite(db *sql.DB) Repository {
	return sqlite.New(db)
}
