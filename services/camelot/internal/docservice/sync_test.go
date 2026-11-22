package docservice_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/mrdhira/camelot/services/camelot/internal/docmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/taskservice/taskservicetest"
)

// edit rewrites a harness document with replace applied.
func editDoc(t *testing.T, h *taskservicetest.Harness, name, old, replacement string) {
	t.Helper()
	path := filepath.Join(h.Vault, name)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(b), old, replacement, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
}

// docEvents lists document events as "<doc> <kind> <from>→<to> <actor>", oldest first.
func docEvents(t *testing.T, h *taskservicetest.Harness) []string {
	t.Helper()
	events, err := h.Docs.Events(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]
		to := e.ToValue
		if e.Kind == docmodel.KindEdited {
			to = "<mtime>"
		}
		out = append(out, e.DocID+" "+e.Kind+" "+e.FromValue+"→"+to+" "+e.Actor)
	}
	return out
}

func TestDocSync(t *testing.T) {
	const prd = "docs/prd/PRD001-product.md"
	tests := []struct {
		name     string
		change   func(t *testing.T, h *taskservicetest.Harness)
		expected []string
	}{
		{name: "the first sync is a silent baseline", change: func(*testing.T, *taskservicetest.Harness) {}, expected: nil},
		{
			name:     "a body edit",
			change:   func(t *testing.T, h *taskservicetest.Harness) { editDoc(t, h, prd, "# Product", "# Product, revised") },
			expected: []string{"PRD001 edited →<mtime> file"},
		},
		{
			name: "a status edited in the file",
			change: func(t *testing.T, h *taskservicetest.Harness) {
				editDoc(t, h, prd, "status: draft", "status: in-review")
			},
			expected: []string{"PRD001 status draft→in-review file"},
		},
		{
			name: "a new document",
			change: func(t *testing.T, h *taskservicetest.Harness) {
				if err := os.WriteFile(filepath.Join(h.Vault, "docs/trd.md"), []byte("---\nid: TRD001\ntype: trd\ntitle: T\nstatus: draft\nupdated: 2026-11-22\nserves: [PRD001]\n---\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			expected: []string{"TRD001 created →draft file"},
		},
		{
			name: "a removed document",
			change: func(t *testing.T, h *taskservicetest.Harness) {
				if err := os.Remove(filepath.Join(h.Vault, prd)); err != nil {
					t.Fatal(err)
				}
			},
			expected: []string{"PRD001 removed " + prd + "→ file"},
		},
		{
			name: "a status set through Camelot carries its actor, once",
			change: func(t *testing.T, h *taskservicetest.Harness) {
				if _, err := h.Docs.SetStatus(context.Background(), &docmodel.SetStatusRequest{ID: "PRD001", Status: "approved", Actor: "dhira"}); err != nil {
					t.Fatal(err)
				}
			},
			expected: []string{"PRD001 status draft→approved dhira"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := taskservicetest.New(t)
			ctx := context.Background()
			if err := h.Docs.Sync(ctx); err != nil {
				t.Fatal(err)
			}
			tt.change(t, h)
			for range 2 {
				if err := h.Docs.Sync(ctx); err != nil {
					t.Fatal(err)
				}
			}
			if got := docEvents(t, h); !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("events = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestDocSyncKeepsAnUnreadableDocument(t *testing.T) {
	h := taskservicetest.New(t)
	ctx := context.Background()
	if err := h.Docs.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	editDoc(t, h, "docs/prd/PRD001-product.md", "status: draft", "  status: draft")
	if err := h.Docs.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	editDoc(t, h, "docs/prd/PRD001-product.md", "  status: draft", "status: draft")
	if err := h.Docs.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	if got := docEvents(t, h); len(got) != 0 {
		t.Errorf("a document with a frontmatter error for a while: events = %q, want none (not removed, not re-created)", got)
	}
}

func TestDocSyncConcurrent(t *testing.T) {
	h := taskservicetest.New(t)
	ctx := context.Background()
	if err := h.Docs.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	editDoc(t, h, "docs/prd/PRD001-product.md", "# Product", "# Product, revised")
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if err := h.Docs.Sync(ctx); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if got := docEvents(t, h); len(got) != 1 {
		t.Errorf("8 concurrent syncs of one edit: events = %q, want exactly one", got)
	}
}
