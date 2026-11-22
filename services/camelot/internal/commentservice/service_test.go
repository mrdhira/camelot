package commentservice_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mrdhira/camelot/services/camelot/internal/commentmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/docmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/taskmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/taskservice/taskservicetest"
)

func TestAdd(t *testing.T) {
	tests := []struct {
		name     string
		input    *commentmodel.AddRequest
		expected *commentmodel.Comment
		err      error
	}{
		{name: "on a card, text trimmed", input: &commentmodel.AddRequest{Kind: "card", TargetID: "DITA-001", Body: "  Split the plan in two.\r\n", Actor: "dhira"}, expected: &commentmodel.Comment{Kind: "card", TargetID: "DITA-001", Body: "Split the plan in two.", Actor: "dhira"}},
		{name: "on a document section", input: &commentmodel.AddRequest{Kind: "doc", TargetID: "PRD001", Section: "goals", Body: "Which clicks?", Actor: "dhira"}, expected: &commentmodel.Comment{Kind: "doc", TargetID: "PRD001", Section: "goals", Body: "Which clicks?", Actor: "dhira"}},
		{name: "on a whole document", input: &commentmodel.AddRequest{Kind: "doc", TargetID: "PRD001", Body: "Looks right.", Actor: "dita"}, expected: &commentmodel.Comment{Kind: "doc", TargetID: "PRD001", Body: "Looks right.", Actor: "dita"}},
		{name: "a section that is not a heading", input: &commentmodel.AddRequest{Kind: "doc", TargetID: "PRD001", Section: "nope", Body: "x", Actor: "dhira"}, err: commentmodel.ErrInvalid},
		{name: "a section on a card", input: &commentmodel.AddRequest{Kind: "card", TargetID: "DITA-001", Section: "goals", Body: "x", Actor: "dhira"}, err: commentmodel.ErrInvalid},
		{name: "a missing card", input: &commentmodel.AddRequest{Kind: "card", TargetID: "DITA-404", Body: "x", Actor: "dhira"}, err: taskmodel.ErrNotFound},
		{name: "a missing document", input: &commentmodel.AddRequest{Kind: "doc", TargetID: "PRD404", Body: "x", Actor: "dhira"}, err: docmodel.ErrNotFound},
		{name: "an unknown kind", input: &commentmodel.AddRequest{Kind: "page", TargetID: "x", Body: "x", Actor: "dhira"}, err: commentmodel.ErrInvalid},
		{name: "empty text", input: &commentmodel.AddRequest{Kind: "card", TargetID: "DITA-001", Body: " \n ", Actor: "dhira"}, err: commentmodel.ErrInvalid},
		{name: "4,000 characters", input: &commentmodel.AddRequest{Kind: "card", TargetID: "DITA-001", Body: strings.Repeat("é", 4000), Actor: "dhira"}, expected: &commentmodel.Comment{Kind: "card", TargetID: "DITA-001", Body: strings.Repeat("é", 4000), Actor: "dhira"}},
		{name: "4,001 characters", input: &commentmodel.AddRequest{Kind: "card", TargetID: "DITA-001", Body: strings.Repeat("é", 4001), Actor: "dhira"}, err: commentmodel.ErrInvalid},
		{name: "no actor", input: &commentmodel.AddRequest{Kind: "card", TargetID: "DITA-001", Body: "x"}, err: commentmodel.ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := taskservicetest.New(t)
			h.Seed(t)
			got, err := h.Comments.Add(context.Background(), tt.input)
			if !errors.Is(err, tt.err) {
				t.Fatalf("Add() error = %v, want %v", err, tt.err)
			}
			if tt.err != nil {
				return
			}
			if got.ID == 0 || got.CreatedAt.IsZero() || got.Kind != tt.expected.Kind || got.TargetID != tt.expected.TargetID ||
				got.Section != tt.expected.Section || got.Body != tt.expected.Body || got.Actor != tt.expected.Actor {
				t.Errorf("Add() = %+v, want %+v", got, tt.expected)
			}
		})
	}
}

func TestThread(t *testing.T) {
	type step struct {
		actor    string
		resolve  int
		resolver string
	}
	tests := []struct {
		name     string
		input    []step
		expected commentmodel.Thread
		rows     int
	}{
		{name: "no comments", expected: commentmodel.Thread{}, rows: 0},
		{name: "Dhira's comment awaits a reply", input: []step{{actor: "dhira"}}, expected: commentmodel.Thread{Open: 1, AwaitsReply: true}, rows: 1},
		{name: "an agent replied", input: []step{{actor: "dhira"}, {actor: "go-engineer"}}, expected: commentmodel.Thread{Open: 2}, rows: 2},
		{name: "the agent resolved Dhira's comment", input: []step{{actor: "dhira"}, {actor: "go-engineer", resolve: 1, resolver: "go-engineer"}}, expected: commentmodel.Thread{}, rows: 2},
		{name: "Dhira resolved the agent's reply: his own comment was answered", input: []step{{actor: "dhira"}, {actor: "go-engineer"}, {actor: "dhira", resolve: 2, resolver: "dhira"}}, expected: commentmodel.Thread{Open: 1}, rows: 3},
		{name: "Dhira wrote again after the reply", input: []step{{actor: "dhira"}, {actor: "go-engineer"}, {actor: "dhira"}}, expected: commentmodel.Thread{Open: 3, AwaitsReply: true}, rows: 3},
		{name: "resolving twice changes nothing", input: []step{{actor: "dhira"}, {actor: "dita", resolve: 1, resolver: "dita"}, {actor: "go-engineer", resolve: 1, resolver: "dita"}}, expected: commentmodel.Thread{}, rows: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := taskservicetest.New(t)
			h.Seed(t)
			ctx := context.Background()
			var ids []int64
			for _, s := range tt.input {
				if s.resolve > 0 {
					got, err := h.Comments.Resolve(ctx, &commentmodel.ResolveRequest{ID: ids[s.resolve-1], Note: "done", Actor: s.actor})
					if err != nil {
						t.Fatal(err)
					}
					if got.ResolvedBy != s.resolver {
						t.Errorf("ResolvedBy = %q, want %q", got.ResolvedBy, s.resolver)
					}
					continue
				}
				c, err := h.Comments.Add(ctx, &commentmodel.AddRequest{Kind: "card", TargetID: "DITA-001", Body: "text", Actor: s.actor})
				if err != nil {
					t.Fatal(err)
				}
				ids = append(ids, c.ID)
			}
			got, err := h.Comments.Thread(ctx, &commentmodel.ListRequest{Kind: "card", TargetID: "DITA-001"})
			if err != nil {
				t.Fatal(err)
			}
			if got.Open != tt.expected.Open || got.AwaitsReply != tt.expected.AwaitsReply || len(got.Comments) != len(ids) {
				t.Errorf("Thread() open %d awaits %v comments %d; want open %d awaits %v comments %d", got.Open, got.AwaitsReply, len(got.Comments), tt.expected.Open, tt.expected.AwaitsReply, len(ids))
			}
			rows, err := h.Comments.Changes(ctx, &commentmodel.Filter{Kind: "card"})
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != tt.rows {
				t.Errorf("stored rows = %d, want %d", len(rows), tt.rows)
			}
			open, err := h.Comments.Thread(ctx, &commentmodel.ListRequest{Kind: "card", TargetID: "DITA-001", OpenOnly: true})
			if err != nil {
				t.Fatal(err)
			}
			if len(open.Comments) != tt.expected.Open {
				t.Errorf("open only: %d comments, want %d", len(open.Comments), tt.expected.Open)
			}
			sums, err := h.Comments.Summaries(ctx, "card")
			if err != nil {
				t.Fatal(err)
			}
			if sums["DITA-001"] != (commentmodel.Summary{Open: tt.expected.Open, AwaitsReply: tt.expected.AwaitsReply}) {
				t.Errorf("Summaries() = %+v", sums["DITA-001"])
			}
		})
	}
}

func TestResolve(t *testing.T) {
	h := taskservicetest.New(t)
	h.Seed(t)
	ctx := context.Background()
	c, err := h.Comments.Add(ctx, &commentmodel.AddRequest{Kind: "card", TargetID: "DITA-001", Body: "x", Actor: "dhira"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := h.Comments.Resolve(ctx, &commentmodel.ResolveRequest{ID: c.ID, Note: "split into two cards", Actor: "go-engineer"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Open() || got.ResolvedBy != "go-engineer" || got.ResolvedNote != "split into two cards" {
		t.Errorf("Resolve() = %+v", got)
	}
	tests := []struct {
		name  string
		input *commentmodel.ResolveRequest
		err   error
	}{
		{name: "a missing comment", input: &commentmodel.ResolveRequest{ID: 999, Actor: "dita"}, err: commentmodel.ErrNotFound},
		{name: "a resolution is not a comment", input: &commentmodel.ResolveRequest{ID: c.ID + 1, Actor: "dita"}, err: commentmodel.ErrInvalid},
		{name: "no actor", input: &commentmodel.ResolveRequest{ID: c.ID}, err: commentmodel.ErrInvalid},
	}
	for _, request := range []*commentmodel.ListRequest{{Kind: "card", TargetID: "DITA-404"}, {Kind: "doc", TargetID: "PRD404"}} {
		if _, err := h.Comments.Thread(ctx, request); err == nil {
			t.Errorf("Thread(%s %s) of a missing target: nil error", request.Kind, request.TargetID)
		}
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := h.Comments.Resolve(ctx, tt.input); !errors.Is(err, tt.err) {
				t.Errorf("Resolve() error = %v, want %v", err, tt.err)
			}
		})
	}
}
