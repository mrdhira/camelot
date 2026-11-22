// Package sqlite stores document snapshots and change events.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mrdhira/camelot/services/camelot/internal/docmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/pkg/sqlitedb"
)

// Store is the document history on one database.
type Store struct {
	db *sql.DB
}

// New returns the store on db.
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// SnapshotSelect returns every snapshot.
func (s *Store) SnapshotSelect(ctx context.Context) ([]*docmodel.Snapshot, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT doc_id, path, title, type, status, content_sha256, modified_at, synced_at FROM doc_snapshots ORDER BY doc_id")
	if err != nil {
		return nil, fmt.Errorf("doc snapshot select: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []*docmodel.Snapshot
	for rows.Next() {
		sn := &docmodel.Snapshot{}
		if err := rows.Scan(&sn.DocID, &sn.Path, &sn.Title, &sn.Type, &sn.Status, &sn.Hash, &sn.ModifiedAt, &sn.SyncedAt); err != nil {
			return nil, fmt.Errorf("doc snapshot select: %w", err)
		}
		out = append(out, sn)
	}
	return out, rows.Err()
}

// SnapshotSync applies each change whose base still matches.
func (s *Store) SnapshotSync(ctx context.Context, changes []*docmodel.Change, write func() error) ([]*docmodel.Event, error) {
	var written []*docmodel.Event
	err := sqlitedb.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		written = nil
		if write != nil {
			if err := write(); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO doc_sync_state (id, baselined_at) VALUES (1, ?) ON CONFLICT (id) DO NOTHING", time.Now().UTC()); err != nil {
			return fmt.Errorf("doc sync state: %w", err)
		}
		for _, c := range changes {
			var current string
			err := tx.QueryRowContext(ctx, "SELECT content_sha256 FROM doc_snapshots WHERE doc_id = ?", c.DocID).Scan(&current)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("doc snapshot sync: %w", err)
			}
			if current != c.BaseHash {
				continue
			}
			if c.Snapshot == nil {
				if _, err := tx.ExecContext(ctx, "DELETE FROM doc_snapshots WHERE doc_id = ?", c.DocID); err != nil {
					return fmt.Errorf("doc snapshot sync: %w", err)
				}
			} else if err := upsert(ctx, tx, c.Snapshot); err != nil {
				return err
			}
			for _, e := range c.Events {
				res, err := tx.ExecContext(ctx, "INSERT INTO doc_events (doc_id, actor, kind, from_value, to_value, created_at) VALUES (?, ?, ?, ?, ?, ?)",
					e.DocID,
					e.Actor,
					e.Kind,
					e.FromValue,
					e.ToValue,
					e.CreatedAt,
				)
				if err != nil {
					return fmt.Errorf("doc events: %w", err)
				}
				if e.ID, err = res.LastInsertId(); err != nil {
					return fmt.Errorf("doc events: %w", err)
				}
				written = append(written, e)
			}
		}
		return nil
	})
	return written, err
}

// Baselined reports whether the first sync was recorded.
func (s *Store) Baselined(ctx context.Context) (bool, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM doc_sync_state").Scan(&n); err != nil {
		return false, fmt.Errorf("doc sync state: %w", err)
	}
	return n > 0, nil
}

// DocEventMaxID returns the highest event ID.
func (s *Store) DocEventMaxID(ctx context.Context) (int64, error) {
	var id int64
	if err := s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(id), 0) FROM doc_events").Scan(&id); err != nil {
		return 0, fmt.Errorf("doc event max id: %w", err)
	}
	return id, nil
}

// upsert writes one snapshot.
func upsert(ctx context.Context, tx *sql.Tx, sn *docmodel.Snapshot) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO doc_snapshots (doc_id, path, title, type, status, content_sha256, modified_at, synced_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (doc_id) DO UPDATE SET path = excluded.path, title = excluded.title, type = excluded.type, status = excluded.status,
			content_sha256 = excluded.content_sha256, modified_at = excluded.modified_at, synced_at = excluded.synced_at`,
		sn.DocID,
		sn.Path,
		sn.Title,
		sn.Type,
		sn.Status,
		sn.Hash,
		sn.ModifiedAt,
		sn.SyncedAt,
	)
	if err != nil {
		return fmt.Errorf("doc snapshot sync: %w", err)
	}
	return nil
}

// DocEventSelect returns events matching filter, newest first.
func (s *Store) DocEventSelect(ctx context.Context, filter *docmodel.EventFilter) ([]*docmodel.Event, error) {
	q := "SELECT id, doc_id, actor, kind, from_value, to_value, created_at FROM doc_events"
	var where []string
	var args []any
	if filter != nil && filter.DocID != "" {
		where = append(where, "doc_id = ?")
		args = append(args, filter.DocID)
	}
	if filter != nil && !filter.Since.IsZero() {
		where = append(where, "created_at > ?")
		args = append(args, filter.Since)
	}
	if filter != nil && filter.AfterID > 0 {
		where = append(where, "id > ?")
		args = append(args, filter.AfterID)
	}
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ") //nolint:gosec // fixed fragments; values are bound parameters
	}
	q += " ORDER BY created_at DESC, id DESC"
	if filter != nil && filter.Limit > 0 {
		q += " LIMIT " + strconv.Itoa(filter.Limit) //nolint:gosec // an int, formatted here
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("doc event select: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []*docmodel.Event
	for rows.Next() {
		e := &docmodel.Event{}
		if err := rows.Scan(&e.ID, &e.DocID, &e.Actor, &e.Kind, &e.FromValue, &e.ToValue, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("doc event select: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
