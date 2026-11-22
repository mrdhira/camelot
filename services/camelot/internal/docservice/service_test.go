package docservice_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mrdhira/camelot/services/camelot/internal/docmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/taskservice/taskservicetest"
)

func write(t *testing.T, vault, name, content string) {
	t.Helper()
	path := filepath.Join(vault, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestList(t *testing.T) {
	type expected struct {
		ids      []string
		problems []string
	}
	tests := []struct {
		name     string
		input    map[string]string
		expected expected
	}{
		{name: "the chain order: BRD before PRD; plain Markdown is not a document", expected: expected{ids: []string{"BRD001", "PRD001"}}},
		{
			name: "broken files and links are problems, never dropped silently",
			input: map[string]string{
				"docs/trd/TRD001.md": "---\nid: TRD001\ntype: trd\ntitle: T\nstatus: draft\nserves: [PRD001, PRD404]\n---\n",
				"docs/trd/TRD002.md": "---\nid: TRD002\ntype: trd\ntitle: T\nstatus: unknown\n---\n",
				"docs/x/dup.md":      "---\nid: PRD001\ntype: prd\ntitle: Copy\nstatus: draft\n---\n",
				"docs/x/bad.md":      "---\nid: BAD\ntype: brd\n  indented: x\n---\n",
				"docs/x/okr.md":      "---\nid: OKR-1\ntype: okr\ntitle: O\nstatus: draft\nserves: [BRD001]\n---\n",
			},
			expected: expected{
				ids: []string{"OKR-1", "BRD001", "PRD001", "TRD001", "TRD002"},
				problems: []string{
					"docs/trd/TRD001.md: serves PRD404, which is not a document",
					"docs/trd/TRD002.md: invalid document: status \"unknown\"",
					"docs/x/bad.md: frontmatter: line 4",
					"docs/x/dup.md: duplicate document ID",
					"docs/x/okr.md: a OKR serves a BRD",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := taskservicetest.New(t)
			for name, content := range tt.input {
				write(t, h.Vault, name, content)
			}
			got, err := h.Docs.List(context.Background(), nil)
			if err != nil {
				t.Fatal(err)
			}
			var ids []string
			for _, d := range got.Docs {
				ids = append(ids, d.ID)
			}
			if !reflect.DeepEqual(ids, tt.expected.ids) {
				t.Errorf("ids = %v, want %v", ids, tt.expected.ids)
			}
			if len(got.Problems) != len(tt.expected.problems) {
				t.Fatalf("problems = %+v, want %d", got.Problems, len(tt.expected.problems))
			}
			for _, want := range tt.expected.problems {
				found := false
				for _, p := range got.Problems {
					if strings.HasPrefix(p.Path+": "+p.Message, want) {
						found = true
					}
				}
				if !found {
					t.Errorf("no problem starting %q in %+v", want, got.Problems)
				}
			}
		})
	}
}

func TestGet(t *testing.T) {
	h := taskservicetest.New(t)
	write(t, h.Vault, "docs/trd/TRD001.md", "---\nid: TRD001\ntype: trd\ntitle: T\nstatus: draft\nserves: [PRD001]\n---\n")
	tests := []struct {
		name     string
		input    string
		expected [2][]string
		err      error
	}{
		{name: "the middle of the chain", input: "PRD001", expected: [2][]string{{"BRD001"}, {"TRD001"}}},
		{name: "the bottom walks up to the top", input: "TRD001", expected: [2][]string{{"BRD001", "PRD001"}, nil}},
		{name: "a missing document", input: "PRD404", err: docmodel.ErrNotFound},
		{name: "a malformed ID", input: "prd 1", err: docmodel.ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain, err := h.Docs.Get(context.Background(), &docmodel.GetRequest{ID: tt.input})
			if tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Fatalf("Get() error = %v, want %v", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			ids := func(docs []*docmodel.Doc) []string {
				var out []string
				for _, d := range docs {
					out = append(out, d.ID)
				}
				return out
			}
			if got := [2][]string{ids(chain.Up), ids(chain.Down)}; !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("chain up/down = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSetStatus(t *testing.T) {
	tests := []struct {
		name     string
		input    *docmodel.SetStatusRequest
		expected error
	}{
		{name: "a status by its label", input: &docmodel.SetStatusRequest{ID: "PRD001", Status: "In review", Actor: "dhira"}},
		{name: "an unknown status", input: &docmodel.SetStatusRequest{ID: "PRD001", Status: "done", Actor: "dhira"}, expected: docmodel.ErrInvalid},
		{name: "a missing document", input: &docmodel.SetStatusRequest{ID: "PRD404", Status: "approved", Actor: "dhira"}, expected: docmodel.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := taskservicetest.New(t)
			doc, err := h.Docs.SetStatus(context.Background(), tt.input)
			if !errors.Is(err, tt.expected) {
				t.Fatalf("SetStatus() error = %v, want %v", err, tt.expected)
			}
			if tt.expected != nil {
				return
			}
			content, err := os.ReadFile(filepath.Join(h.Vault, doc.Path))
			if err != nil {
				t.Fatal(err)
			}
			today := time.Now().UTC().Format(docmodel.DateLayout)
			want := strings.Replace(strings.Replace(taskservicetest.Docs[doc.Path], "status: draft", "status: in-review", 1), "updated: 2026-11-02", "updated: "+today, 1)
			if string(content) != want {
				t.Errorf("file = %q, want %q (only status and updated changed)", content, want)
			}
		})
	}
}
