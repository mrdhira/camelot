// Package sqlite stores comments.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/mrdhira/camelot/services/camelot/internal/commentmodel"
)

// Store is the comments on one database.
type Store struct {
	db *sql.DB
}

// New returns the store on db.
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

const columns = "id, kind, target_id, section, body, actor, created_at, COALESCE(resolves_id, 0)"

// CommentInsert stores c; resolving an already resolved comment changes nothing.
func (s *Store) CommentInsert(ctx context.Context, c *commentmodel.Comment) error {
	var resolves any
	if c.ResolvesID != 0 {
		resolves = c.ResolvesID
	}
	res, err := s.db.ExecContext(ctx, "INSERT INTO comments (kind, target_id, section, body, actor, created_at, resolves_id) VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT (resolves_id) DO NOTHING",
		c.Kind, c.TargetID, c.Section, c.Body, c.Actor, c.CreatedAt, resolves)
	if err != nil {
		return fmt.Errorf("comment insert: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil
	}
	if c.ID, err = res.LastInsertId(); err != nil {
		return fmt.Errorf("comment insert: %w", err)
	}
	return nil
}

// CommentGet returns one row.
func (s *Store) CommentGet(ctx context.Context, id int64) (*commentmodel.Comment, error) {
	c := &commentmodel.Comment{}
	err := s.db.QueryRowContext(ctx, "SELECT "+columns+" FROM comments WHERE id = ?", id).
		Scan(&c.ID, &c.Kind, &c.TargetID, &c.Section, &c.Body, &c.Actor, &c.CreatedAt, &c.ResolvesID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %d", commentmodel.ErrNotFound, id)
	}
	if err != nil {
		return nil, fmt.Errorf("comment get: %w", err)
	}
	return c, nil
}

// CommentSelect returns the rows matching the filter, oldest first.
func (s *Store) CommentSelect(ctx context.Context, filter *commentmodel.Filter) ([]*commentmodel.Comment, error) {
	var where []string
	var args []any
	if filter.Kind != "" {
		where, args = append(where, "kind = ?"), append(args, filter.Kind)
	}
	if filter.TargetID != "" {
		where, args = append(where, "target_id = ?"), append(args, filter.TargetID)
	}
	if filter.AfterID > 0 {
		where, args = append(where, "id > ?"), append(args, filter.AfterID)
	}
	if !filter.SinceAt.IsZero() {
		where, args = append(where, "created_at > ?"), append(args, filter.SinceAt)
	}
	query := "SELECT " + columns + " FROM comments"
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	rows, err := s.db.QueryContext(ctx, query+" ORDER BY id", args...)
	if err != nil {
		return nil, fmt.Errorf("comment select: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []*commentmodel.Comment
	for rows.Next() {
		c := &commentmodel.Comment{}
		if err := rows.Scan(&c.ID, &c.Kind, &c.TargetID, &c.Section, &c.Body, &c.Actor, &c.CreatedAt, &c.ResolvesID); err != nil {
			return nil, fmt.Errorf("comment select: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CommentMaxID returns the highest row ID.
func (s *Store) CommentMaxID(ctx context.Context) (int64, error) {
	var id int64
	if err := s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(id), 0) FROM comments").Scan(&id); err != nil {
		return 0, fmt.Errorf("comment max id: %w", err)
	}
	return id, nil
}
