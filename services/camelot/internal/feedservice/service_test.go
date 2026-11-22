package feedservice_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/mrdhira/camelot/services/camelot/internal/feedmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/taskmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/taskservice/taskservicetest"
)

// edit rewrites a harness document with replace applied.
func edit(t *testing.T, h *taskservicetest.Harness, name, old, replacement string) {
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

func TestFeed(t *testing.T) {
	h := taskservicetest.New(t)
	ctx := context.Background()
	h.Seed(t)
	feed, err := h.Feed.Feed(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !feed.FirstLook || len(feed.Items) != 2 {
		t.Fatalf("first feed = %+v, want the first look with M1 and DITA-001", feed)
	}
	stamp, err := h.Feed.Stamp(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Feed.MarkSeen(ctx, &feedmodel.SeenRequest{Stamp: feed.Stamp.String()}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Tasks.Update(ctx, &taskmodel.UpdateRequest{ID: "DITA-001", Status: ptr("wip"), Actor: "go-engineer"}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Tasks.Update(ctx, &taskmodel.UpdateRequest{ID: "DITA-001", Title: ptr("Seed card, renamed"), Actor: "go-engineer"}); err != nil {
		t.Fatal(err)
	}
	edit(t, h, "docs/prd/PRD001-product.md", "# Product", "# Product, revised")

	feed, err = h.Feed.Feed(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, it := range feed.Items {
		got = append(got, it.Kind+" "+it.ID+" "+it.Title+" changes="+itoa(it.Changes()))
	}
	want := []string{"doc PRD001 Product: links changes=1", "card DITA-001 Seed card, renamed changes=2"}
	if !reflect.DeepEqual(got, want) || feed.FirstLook {
		t.Errorf("feed after seen = %q (first look %v), want %q", got, feed.FirstLook, want)
	}
	next, err := h.Feed.Stamp(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if next == stamp || next.String() == stamp.String() {
		t.Errorf("stamp %q did not change after card and document changes", next)
	}
	if err := h.Feed.MarkSeen(ctx, &feedmodel.SeenRequest{Stamp: "c0-d0"}); err != nil {
		t.Fatal(err)
	}
	if again, _ := h.Feed.Feed(ctx); len(again.Items) != 2 {
		t.Errorf("an older marker moved the seen marker back: %d items", len(again.Items))
	}
	if err := h.Feed.MarkSeen(ctx, &feedmodel.SeenRequest{Stamp: "x"}); err == nil {
		t.Errorf("MarkSeen of a malformed stamp: nil error")
	}
	if err := h.Feed.MarkSeen(ctx, &feedmodel.SeenRequest{Stamp: feed.Stamp.String()}); err != nil {
		t.Fatal(err)
	}
	if empty, _ := h.Feed.Feed(ctx); len(empty.Items) != 0 {
		t.Errorf("after mark seen: %d items, want 0", len(empty.Items))
	}
}

func ptr[T any](v T) *T { return &v }

func itoa(n int) string { return strconv.Itoa(n) }
