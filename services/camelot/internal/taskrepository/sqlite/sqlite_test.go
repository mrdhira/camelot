package sqlite_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/mrdhira/camelot/services/camelot/internal/pkg/sqlitedb"
	"github.com/mrdhira/camelot/services/camelot/internal/taskmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/taskrepository"
)

var t0 = time.Date(2026, 11, 1, 9, 0, 0, 123456789, time.UTC)

// store returns the repository on a fresh in-memory database with cards A
// (no links) and B (depends on A), both last updated at t0.
func store(t *testing.T) taskrepository.Repository {
	t.Helper()
	db, err := sqlitedb.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := sqlitedb.Migrate(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), db, taskrepository.Migrations); err != nil {
		t.Fatal(err)
	}
	repo := taskrepository.NewSQLite(db)
	for _, c := range []*taskmodel.Card{
		{ID: "A", Title: "a", Type: taskmodel.TypeTask, Status: taskmodel.StatusBacklog, Deps: []string{}, Docs: []string{}, CreatedAt: t0, UpdatedAt: t0},
		{ID: "B", Title: "b", Type: taskmodel.TypeTask, Status: taskmodel.StatusBacklog, Deps: []string{"A"}, Docs: []string{}, CreatedAt: t0, UpdatedAt: t0},
	} {
		if err := repo.CardInsert(context.Background(), c, []*taskmodel.Event{taskmodel.NewEvent(c.ID, "test", taskmodel.FieldCreated, "", string(c.Status), "", t0)}); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

// eventCount counts a card's events.
func eventCount(t *testing.T, repo taskrepository.Repository, id string) int {
	t.Helper()
	events, err := repo.CardEventSelect(context.Background(), &taskmodel.EventFilter{CardID: id})
	if err != nil {
		t.Fatal(err)
	}
	return len(events)
}

func TestCardUpdate(t *testing.T) {
	now := t0.Add(time.Hour)
	type input struct {
		id     string
		deps   []string
		readAt time.Time
		events []*taskmodel.Event
	}
	type expected struct {
		updated bool
		err     error
		events  int
		status  taskmodel.Status
	}
	move := func(id string) []*taskmodel.Event {
		return []*taskmodel.Event{taskmodel.NewEvent(id, "test", taskmodel.FieldStatus, "backlog", "wip", "", now)}
	}
	tests := []struct {
		name     string
		input    input
		expected expected
	}{
		{name: "the stored updated_at matches: updated with its event", input: input{id: "A", readAt: t0, events: move("A")}, expected: expected{updated: true, events: 2, status: taskmodel.StatusWIP}},
		{name: "a stale read: nothing written, nil", input: input{id: "A", readAt: t0.Add(-time.Nanosecond), events: move("A")}, expected: expected{events: 1, status: taskmodel.StatusBacklog}},
		{name: "a failed event insert rolls the card back", input: input{id: "A", readAt: t0, events: []*taskmodel.Event{nil}}, expected: expected{err: errors.New("any"), events: 1, status: taskmodel.StatusBacklog}},
		{name: "a dependency cycle is refused inside the transaction", input: input{id: "A", deps: []string{"B"}, readAt: t0, events: move("A")}, expected: expected{err: taskmodel.ErrInvalidLink, events: 1, status: taskmodel.StatusBacklog}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := store(t)
			ctx := context.Background()
			card, err := repo.CardSelectByID(ctx, tt.input.id)
			if err != nil {
				t.Fatal(err)
			}
			if !card.UpdatedAt.Equal(t0) {
				t.Fatalf("updated_at read back as %v, want %v (nanoseconds must survive the round trip)", card.UpdatedAt, t0)
			}
			card.Status = taskmodel.StatusWIP
			if tt.input.deps != nil {
				card.Deps = tt.input.deps
			}
			card.UpdatedAt = now
			got, err := repo.CardUpdate(ctx, card, tt.input.readAt, tt.input.events)
			switch {
			case tt.expected.err == nil && err != nil:
				t.Fatalf("CardUpdate() error = %v", err)
			case errors.Is(tt.expected.err, taskmodel.ErrInvalidLink) && !errors.Is(err, taskmodel.ErrInvalidLink):
				t.Fatalf("CardUpdate() error = %v, want ErrInvalidLink", err)
			case tt.expected.err != nil && err == nil:
				t.Fatalf("CardUpdate() error = nil, want an error")
			}
			if (got != nil) != tt.expected.updated {
				t.Errorf("CardUpdate() returned card = %v, want updated %v", got != nil, tt.expected.updated)
			}
			stored, err := repo.CardSelectByID(ctx, tt.input.id)
			if err != nil {
				t.Fatal(err)
			}
			if stored.Status != tt.expected.status {
				t.Errorf("stored status = %s, want %s", stored.Status, tt.expected.status)
			}
			if n := eventCount(t, repo, tt.input.id); n != tt.expected.events {
				t.Errorf("events = %d, want %d", n, tt.expected.events)
			}
		})
	}
}

func TestCardDelete(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		deleted  bool
		expected error
	}{
		{name: "a card nothing depends on", input: "B", deleted: true},
		{name: "a card another depends on is refused, its dependent's link kept", input: "A", expected: taskmodel.ErrInvalidLink},
		{name: "a missing card", input: "Z"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := store(t)
			ctx := context.Background()
			deleted, err := repo.CardDelete(ctx, tt.input, t0.Add(time.Hour), taskmodel.NewEvent(tt.input, "test", taskmodel.FieldDeleted, "", "", "", t0.Add(time.Hour)))
			if !errors.Is(err, tt.expected) {
				t.Fatalf("CardDelete() error = %v, want %v", err, tt.expected)
			}
			if deleted != tt.deleted {
				t.Errorf("CardDelete() = %v, want %v", deleted, tt.deleted)
			}
			b, err := repo.CardSelectByID(ctx, "B")
			if err != nil {
				t.Fatal(err)
			}
			if tt.input == "A" && (b == nil || len(b.Deps) != 1) {
				t.Errorf("B's dependency on A = %v, want kept", b)
			}
		})
	}
}
