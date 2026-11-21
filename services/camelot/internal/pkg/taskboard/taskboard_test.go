package taskboard_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mrdhira/camelot/services/camelot/internal/pkg/taskboard"
)

const board = "# TASK.md\n\n## Milestones\n\n| # | Milestone | Spec | Status |\n|---|---|---|---|\n" +
	"| M1 | Identity | TRD001 | ✅ done 2026-11-08 |\n" +
	"| M3 | Payments | TRD003 | 🔄 P0–P5 done |\n\n" +
	"## Backlog\n\n" +
	"- [WIG-081] One Go module — research for discussion → `WIG-081-go-single-module/RESEARCH.md`\n\n" +
	"### M3 — Payments\n" +
	"- [WIG-073] Our own payment simulation — later\n\n" +
	"### M5 — Security\n" +
	"- [WIG-006] Login rate limiting — gate\n\n" +
	"### Tech debt (code, tests, UI polish)\n" +
	"- [WIG-068] Per-merchant time zone — later\n\n" +
	"## Review\n" +
	"- [WIG-057] Redis + Valkey — waiting for Dhira's picks → `WIG-057-redis-valkey/REPORT.md`\n\n" +
	"## Done\n" +
	"- [WIG-T032d+c, WIG-030] Card A: lists — done 2026-11-06 (b0a6a26) → `_archive/WIG-T032d-searchable-lists/`\n" +
	"- not a card line\n"

func TestParse(t *testing.T) {
	b, err := taskboard.Parse(strings.NewReader(board))
	if err != nil {
		t.Fatal(err)
	}
	type card struct {
		ID, Milestone, Type, Status, Folder, Waiting string
		Also                                         []string
		DoneOn                                       time.Time
	}
	var got []card
	for _, c := range b.Cards {
		got = append(got, card{c.ID, c.Milestone, c.Type, c.Status, c.Folder, c.Waiting, c.AlsoIDs, c.DoneOn})
	}
	expected := []card{
		{ID: "WIG-081", Type: "research", Status: "backlog", Folder: "tasks/WIG-081-go-single-module"},
		{ID: "WIG-073", Milestone: "M3", Type: "task", Status: "backlog"},
		{ID: "WIG-006", Milestone: "M5", Type: "security-debt", Status: "backlog"},
		{ID: "WIG-068", Type: "tech-debt", Status: "backlog"},
		{ID: "WIG-057", Type: "task", Status: "review", Folder: "tasks/WIG-057-redis-valkey", Waiting: "see the imported description"},
		{ID: "WIG-T032d", Type: "task", Status: "done", Folder: "tasks/_archive/WIG-T032d-searchable-lists", Also: []string{"WIG-030"}, DoneOn: time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)},
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("cards =\n%+v\nwant\n%+v", got, expected)
	}
	if len(b.Milestones) != 2 || b.Milestones[0].Status != "done" || b.Milestones[1].Status != "wip" {
		t.Errorf("milestones = %+v", b.Milestones)
	}
	if len(b.Skipped) != 0 {
		t.Errorf("skipped = %q, want none (a bullet without [CODE] is not a card line)", b.Skipped)
	}
}
