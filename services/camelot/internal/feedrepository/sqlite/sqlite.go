// Package sqlite stores the feed's seen marker.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/mrdhira/camelot/services/camelot/internal/feedmodel"
)

// Store is the seen marker on one database.
type Store struct {
	db *sql.DB
}

// New returns the store on db.
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// SeenSelect returns the marker; nil when never set.
func (s *Store) SeenSelect(ctx context.Context) (*feedmodel.Seen, error) {
	seen := &feedmodel.Seen{}
	err := s.db.QueryRowContext(ctx, "SELECT card_event_id, doc_event_id, comment_id, seen_at FROM feed_seen WHERE id = 1").
		Scan(&seen.Stamp.CardEventID, &seen.Stamp.DocEventID, &seen.Stamp.CommentID, &seen.At)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("feed seen select: %w", err)
	}
	return seen, nil
}

// SeenUpdate records the marker; each event ID only moves forward.
func (s *Store) SeenUpdate(ctx context.Context, seen *feedmodel.Seen) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO feed_seen (id, card_event_id, doc_event_id, comment_id, seen_at) VALUES (1, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			card_event_id = MAX(feed_seen.card_event_id, excluded.card_event_id),
			doc_event_id = MAX(feed_seen.doc_event_id, excluded.doc_event_id),
			comment_id = MAX(feed_seen.comment_id, excluded.comment_id),
			seen_at = excluded.seen_at`,
		seen.Stamp.CardEventID,
		seen.Stamp.DocEventID,
		seen.Stamp.CommentID,
		seen.At,
	)
	if err != nil {
		return fmt.Errorf("feed seen update: %w", err)
	}
	return nil
}
