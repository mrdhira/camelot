// Package taskservicetest builds the real card and document services on
// an in-memory database and a temporary vault, for the tests of the
// services, the handlers and the MCP server.
package taskservicetest

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mrdhira/camelot/services/camelot/internal/commentrepository"
	"github.com/mrdhira/camelot/services/camelot/internal/commentservice"
	"github.com/mrdhira/camelot/services/camelot/internal/docrepository"
	"github.com/mrdhira/camelot/services/camelot/internal/docservice"
	"github.com/mrdhira/camelot/services/camelot/internal/feedrepository"
	"github.com/mrdhira/camelot/services/camelot/internal/feedservice"
	"github.com/mrdhira/camelot/services/camelot/internal/pkg/sqlitedb"
	"github.com/mrdhira/camelot/services/camelot/internal/taskmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/taskrepository"
	"github.com/mrdhira/camelot/services/camelot/internal/taskservice"
)

// DocRoot is the document root inside the harness's vault.
const DocRoot = "docs"

// Docs are the documents the harness's vault holds: PRD001 serves BRD001.
var Docs = map[string]string{
	"docs/brd/BRD001-business.md": "---\nid: BRD001\ntype: brd\ntitle: Business case\nstatus: approved\nowner: Dhira\nupdated: 2026-11-01\n---\n# Business case\n",
	"docs/prd/PRD001-product.md":  "---\nid: PRD001\ntype: prd\ntitle: \"Product: links\"\nstatus: draft\nowner: Dhira\nupdated: 2026-11-02\nserves: [BRD001]\n---\n# Product\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n## Goals\n\nFewer clicks.\n",
	"docs/notes.md":               "# Not a document: no frontmatter\n",
}

// Harness is the services on a fresh database and vault.
type Harness struct {
	Vault    string
	Repo     taskrepository.Repository
	Tasks    taskservice.Service
	Docs     docservice.Service
	Feed     feedservice.Service
	Comments commentservice.Service
}

// New returns a harness on an in-memory database and a temporary vault holding Docs.
func New(t testing.TB) *Harness {
	t.Helper()
	db, err := sqlitedb.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, set := range []sqlitedb.MigrationSet{taskrepository.Migrations, docrepository.Migrations, feedrepository.Migrations, commentrepository.Migrations} {
		if err := sqlitedb.Migrate(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), db, set); err != nil {
			t.Fatal(err)
		}
	}
	vault := t.TempDir()
	for name, content := range Docs {
		path := filepath.Join(vault, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	docSvc := docservice.New(docservice.Config{}, docrepository.NewMarkdown(vault, []string{DocRoot}), docrepository.NewSQLite(db))
	repo := taskrepository.NewSQLite(db)
	tasks := taskservice.New(repo, taskrepository.NewFolders(vault), docSvc)
	comments := commentservice.New(commentrepository.NewSQLite(db), tasks, docSvc)
	return &Harness{Vault: vault, Repo: repo, Tasks: tasks, Docs: docSvc, Feed: feedservice.New(feedrepository.NewSQLite(db), tasks, docSvc, comments), Comments: comments}
}

// Card stores a card with the given history straight through the
// repository, so tests set every event's time. Each move is a status and
// the time it was entered, in order; the first is the creation.
func (h *Harness) Card(t testing.TB, card *taskmodel.Card, moves ...Move) {
	t.Helper()
	if card.Type == "" {
		card.Type = taskmodel.TypeTask
	}
	if card.Deps == nil {
		card.Deps = []string{}
	}
	if card.Docs == nil {
		card.Docs = []string{}
	}
	var events []*taskmodel.Event
	for i, m := range moves {
		field, from := taskmodel.FieldStatus, ""
		if i == 0 {
			field = taskmodel.FieldCreated
		} else {
			from = string(moves[i-1].Status)
		}
		if m.Imported {
			field = taskmodel.FieldImported
		}
		events = append(events, taskmodel.NewEvent(card.ID, "test", field, from, string(m.Status), m.Note, m.At))
		card.Status = m.Status
		card.CreatedAt, card.UpdatedAt = moves[0].At, m.At
	}
	if err := h.Repo.CardInsert(context.Background(), card, events); err != nil {
		t.Fatal(err)
	}
}

// Move is a status a test card entered and when.
type Move struct {
	Status   taskmodel.Status
	At       time.Time
	Imported bool
	Note     string
}

// Seed creates milestone M1 and card DITA-001 "Seed card" in it.
func (h *Harness) Seed(t testing.TB) {
	t.Helper()
	ctx := context.Background()
	for _, request := range []*taskmodel.CreateRequest{
		{ID: "M1", Title: "Milestone one", Type: string(taskmodel.TypeMilestone), Actor: "dita"},
		{ID: "DITA-001", Title: "Seed card", MilestoneID: "M1", Actor: "dita"},
	} {
		if _, err := h.Tasks.Create(ctx, request); err != nil {
			t.Fatal(err)
		}
	}
}
