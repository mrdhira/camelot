// Package sqlite is the SQLite card store.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mrdhira/camelot/services/camelot/internal/pkg/sqlitedb"
	"github.com/mrdhira/camelot/services/camelot/internal/taskmodel"
)

// Store is the card store on one database.
type Store struct {
	db *sql.DB
}

// New returns the store on db.
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// cardColumns are the cards columns every select reads, in scanCard's order.
const cardColumns = "id, milestone_id, title, type, status, assignee, waiting, waiting_options, folder, description, created_at, updated_at"

// CardInsert stores card, its links and events.
func (s *Store) CardInsert(ctx context.Context, card *taskmodel.Card, events []*taskmodel.Event) error {
	return sqlitedb.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM cards WHERE id = ?", card.ID).Scan(&exists); err != nil {
			return fmt.Errorf("card insert: %w", err)
		}
		if exists > 0 {
			return fmt.Errorf("%w: %s", taskmodel.ErrExists, card.ID)
		}
		_, err := tx.ExecContext(ctx,
			"INSERT INTO cards ("+cardColumns+") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			card.ID,
			card.MilestoneID,
			card.Title,
			card.Type,
			card.Status,
			card.Assignee,
			card.Waiting,
			optionsJSON(card.Options),
			card.Folder,
			card.Description,
			card.CreatedAt,
			card.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("card insert: %w", err)
		}
		if err := writeLinks(ctx, tx, card); err != nil {
			return err
		}
		return insertEvents(ctx, tx, events)
	})
}

// CardSelectByID returns the live card id.
func (s *Store) CardSelectByID(ctx context.Context, id string) (*taskmodel.Card, error) {
	cards, err := s.selectCards(ctx, "id = ?", []any{id})
	if err != nil || len(cards) == 0 {
		return nil, err
	}
	return cards[0], nil
}

// CardSelect returns the live cards matching filter.
func (s *Store) CardSelect(ctx context.Context, filter *taskmodel.CardFilter) ([]*taskmodel.Card, error) {
	var where []string
	var args []any
	if filter != nil {
		if len(filter.Statuses) > 0 {
			where = append(where, "status IN ("+placeholders(len(filter.Statuses))+")")
			for _, st := range filter.Statuses {
				args = append(args, st)
			}
		}
		if len(filter.Types) > 0 {
			where = append(where, "type IN ("+placeholders(len(filter.Types))+")")
			for _, t := range filter.Types {
				args = append(args, t)
			}
		}
		if filter.MilestoneID != "" {
			where = append(where, "milestone_id = ?")
			args = append(args, filter.MilestoneID)
		}
		if filter.Assignee != "" {
			where = append(where, "assignee = ?")
			args = append(args, filter.Assignee)
		}
		if filter.Waiting {
			where = append(where, "waiting <> ''")
		}
		if filter.DocID != "" {
			where = append(where, "id IN (SELECT card_id FROM card_docs WHERE doc_id = ?)")
			args = append(args, filter.DocID)
		}
		if q := strings.TrimSpace(filter.Query); q != "" {
			where = append(where, "(id LIKE ? ESCAPE '\\' OR title LIKE ? ESCAPE '\\')")
			like := "%" + escapeLike(q) + "%"
			args = append(args, like, like)
		}
	}
	return s.selectCards(ctx, strings.Join(where, " AND "), args)
}

// CardUpdate replaces card when it is unchanged since readAt.
func (s *Store) CardUpdate(ctx context.Context, card *taskmodel.Card, readAt time.Time, events []*taskmodel.Event) (*taskmodel.Card, error) {
	updated := false
	err := sqlitedb.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`UPDATE cards SET milestone_id = ?, title = ?, type = ?, status = ?, assignee = ?, waiting = ?, waiting_options = ?, folder = ?, description = ?, updated_at = ?
			WHERE id = ? AND deleted_at IS NULL AND updated_at = ?`,
			card.MilestoneID,
			card.Title,
			card.Type,
			card.Status,
			card.Assignee,
			card.Waiting,
			optionsJSON(card.Options),
			card.Folder,
			card.Description,
			card.UpdatedAt,
			card.ID,
			readAt,
		)
		if err != nil {
			return fmt.Errorf("card update: %w", err)
		}
		if n, err := res.RowsAffected(); err != nil || n == 0 {
			return err
		}
		updated = true
		for _, q := range []string{"DELETE FROM card_deps WHERE card_id = ?", "DELETE FROM card_docs WHERE card_id = ?"} {
			if _, err := tx.ExecContext(ctx, q, card.ID); err != nil {
				return fmt.Errorf("card update: %w", err)
			}
		}
		if err := writeLinks(ctx, tx, card); err != nil {
			return err
		}
		if err := refuseCycle(ctx, tx, card.ID); err != nil {
			return err
		}
		return insertEvents(ctx, tx, events)
	})
	if err != nil || !updated {
		return nil, err
	}
	return card, nil
}

// refuseCycle fails when the dependencies, as written in tx, lead back to id.
func refuseCycle(ctx context.Context, tx *sql.Tx, id string) error {
	var n int
	err := tx.QueryRowContext(ctx, `WITH RECURSIVE reach(id) AS (
			SELECT depends_on_id FROM card_deps WHERE card_id = ?
			UNION
			SELECT d.depends_on_id FROM card_deps d JOIN reach r ON d.card_id = r.id
		) SELECT COUNT(*) FROM reach WHERE id = ?`, id, id).Scan(&n)
	if err != nil {
		return fmt.Errorf("card deps: %w", err)
	}
	if n > 0 {
		return fmt.Errorf("%w: these dependencies would make a cycle back to %s", taskmodel.ErrInvalidLink, id)
	}
	return nil
}

// CardDelete marks id deleted.
func (s *Store) CardDelete(ctx context.Context, id string, at time.Time, event *taskmodel.Event) (bool, error) {
	deleted := false
	err := sqlitedb.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		var holders int
		err := tx.QueryRowContext(ctx, `SELECT
				(SELECT COUNT(*) FROM card_deps WHERE depends_on_id = ?) +
				(SELECT COUNT(*) FROM cards WHERE milestone_id = ? AND deleted_at IS NULL)`, id, id).Scan(&holders)
		if err != nil {
			return fmt.Errorf("card delete: %w", err)
		}
		if holders > 0 {
			return fmt.Errorf("%w: %s is a dependency or milestone of other cards; unlink them first", taskmodel.ErrInvalidLink, id)
		}
		res, err := tx.ExecContext(ctx, "UPDATE cards SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL", at, at, id)
		if err != nil {
			return fmt.Errorf("card delete: %w", err)
		}
		if n, err := res.RowsAffected(); err != nil || n == 0 {
			return err
		}
		deleted = true
		for _, q := range []string{"DELETE FROM card_deps WHERE card_id = ?", "DELETE FROM card_docs WHERE card_id = ?"} {
			if _, err := tx.ExecContext(ctx, q, id); err != nil {
				return fmt.Errorf("card delete: %w", err)
			}
		}
		return insertEvents(ctx, tx, []*taskmodel.Event{event})
	})
	return deleted, err
}

// CardMaxNumber returns the highest N of "<prefix>-N".
func (s *Store) CardMaxNumber(ctx context.Context, prefix string) (int, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id FROM cards WHERE id LIKE ? ESCAPE '\\'", escapeLike(prefix)+"-%")
	if err != nil {
		return 0, fmt.Errorf("card max number: %w", err)
	}
	defer func() { _ = rows.Close() }()
	maxN := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, fmt.Errorf("card max number: %w", err)
		}
		if n, err := strconv.Atoi(strings.TrimPrefix(id, prefix+"-")); err == nil && n > maxN {
			maxN = n
		}
	}
	return maxN, rows.Err()
}

// CardEventSelect returns events matching filter, newest first.
func (s *Store) CardEventSelect(ctx context.Context, filter *taskmodel.EventFilter) ([]*taskmodel.Event, error) {
	q := "SELECT id, card_id, actor, field, from_value, to_value, note, created_at FROM card_events"
	var where []string
	var args []any
	if filter != nil && filter.CardID != "" {
		where = append(where, "card_id = ?")
		args = append(args, filter.CardID)
	}
	if filter != nil && !filter.Since.IsZero() {
		where = append(where, "created_at >= ?")
		args = append(args, filter.Since)
	}
	if filter != nil && filter.AfterID > 0 {
		where = append(where, "id > ?")
		args = append(args, filter.AfterID)
	}
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY created_at DESC, id DESC"
	if filter != nil && filter.Limit > 0 {
		q += " LIMIT " + strconv.Itoa(filter.Limit) //nolint:gosec // an int, formatted here
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("card event select: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var events []*taskmodel.Event
	for rows.Next() {
		e := &taskmodel.Event{}
		if err := rows.Scan(&e.ID, &e.CardID, &e.Actor, &e.Field, &e.FromValue, &e.ToValue, &e.Note, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("card event select: %w", err)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// CardEventMaxID returns the highest event ID.
func (s *Store) CardEventMaxID(ctx context.Context) (int64, error) {
	var id int64
	if err := s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(id), 0) FROM card_events").Scan(&id); err != nil {
		return 0, fmt.Errorf("card event max id: %w", err)
	}
	return id, nil
}

// selectCards reads the live cards matching where with their links.
func (s *Store) selectCards(ctx context.Context, where string, args []any) ([]*taskmodel.Card, error) {
	q := "SELECT " + cardColumns + " FROM cards WHERE deleted_at IS NULL"
	if where != "" {
		q += " AND " + where //nolint:gosec // fixed fragments built in this file; values are bound parameters
	}
	q += " ORDER BY id"
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("card select: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var cards []*taskmodel.Card
	byID := map[string]*taskmodel.Card{}
	for rows.Next() {
		c := &taskmodel.Card{Deps: []string{}, Docs: []string{}}
		var options string
		if err := rows.Scan(&c.ID, &c.MilestoneID, &c.Title, &c.Type, &c.Status, &c.Assignee, &c.Waiting, &options, &c.Folder, &c.Description, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("card select: %w", err)
		}
		if err := json.Unmarshal([]byte(options), &c.Options); err != nil {
			return nil, fmt.Errorf("card select %s: waiting_options: %w", c.ID, err)
		}
		cards = append(cards, c)
		byID[c.ID] = c
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("card select: %w", err)
	}
	if len(cards) == 0 {
		return cards, nil
	}
	if err := s.attach(ctx, "SELECT card_id, depends_on_id FROM card_deps ORDER BY rowid", byID, func(c *taskmodel.Card, v string) { c.Deps = append(c.Deps, v) }); err != nil {
		return nil, err
	}
	if err := s.attach(ctx, "SELECT card_id, doc_id FROM card_docs ORDER BY rowid", byID, func(c *taskmodel.Card, v string) { c.Docs = append(c.Docs, v) }); err != nil {
		return nil, err
	}
	return cards, nil
}

// attach reads (card_id, value) pairs and adds each value to its card.
func (s *Store) attach(ctx context.Context, q string, byID map[string]*taskmodel.Card, add func(*taskmodel.Card, string)) error {
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return fmt.Errorf("card links: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id, v string
		if err := rows.Scan(&id, &v); err != nil {
			return fmt.Errorf("card links: %w", err)
		}
		if c, ok := byID[id]; ok {
			add(c, v)
		}
	}
	return rows.Err()
}

// writeLinks inserts the card's dependencies and document links.
func writeLinks(ctx context.Context, tx *sql.Tx, card *taskmodel.Card) error {
	for _, dep := range card.Deps {
		if _, err := tx.ExecContext(ctx, "INSERT INTO card_deps (card_id, depends_on_id) VALUES (?, ?)", card.ID, dep); err != nil {
			return fmt.Errorf("card deps: %w", err)
		}
	}
	for _, doc := range card.Docs {
		if _, err := tx.ExecContext(ctx, "INSERT INTO card_docs (card_id, doc_id) VALUES (?, ?)", card.ID, doc); err != nil {
			return fmt.Errorf("card docs: %w", err)
		}
	}
	return nil
}

// insertEvents appends events.
func insertEvents(ctx context.Context, tx *sql.Tx, events []*taskmodel.Event) error {
	for _, e := range events {
		if e == nil {
			return errors.New("card events: nil event")
		}
		res, err := tx.ExecContext(ctx,
			"INSERT INTO card_events (card_id, actor, field, from_value, to_value, note, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
			e.CardID,
			e.Actor,
			e.Field,
			e.FromValue,
			e.ToValue,
			e.Note,
			e.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("card events: %w", err)
		}
		if e.ID, err = res.LastInsertId(); err != nil {
			return fmt.Errorf("card events: %w", err)
		}
	}
	return nil
}

// optionsJSON encodes answer options for the waiting_options column.
func optionsJSON(options []string) string {
	if len(options) == 0 {
		return "[]"
	}
	b, err := json.Marshal(options)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// placeholders returns n comma-separated question marks.
func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}

// escapeLike escapes LIKE's wildcards with a backslash.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
