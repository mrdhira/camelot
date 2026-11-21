package insightservice_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/mrdhira/camelot/services/camelot/internal/insightmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/insightservice"
	"github.com/mrdhira/camelot/services/camelot/internal/taskmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/taskservice/taskservicetest"
)

const day = 24 * time.Hour

// fixture stores, relative to now:
//   - D1…D6: Done cards with cycle times of 1…6 days, finished 1 day ago (D6 a bug);
//   - W1: WIP, started 10 days ago, last event 8 days ago (flagged: 10 d > p85; stale);
//   - W2: Review, imported already started (age unknown), waiting on Dhira;
//   - B1: backlog security-debt depending on W1 (blocked);
//   - OLD: imported Done 200 days ago (outside the throughput window);
//   - M1: milestone holding D1 and W1.
func fixture(t *testing.T) (*taskservicetest.Harness, insightservice.Service) {
	h := taskservicetest.New(t)
	now := time.Now().UTC()
	h.Card(t, &taskmodel.Card{ID: "M1", Title: "Milestone", Type: taskmodel.TypeMilestone}, taskservicetest.Move{Status: taskmodel.StatusTodo, At: now.Add(-30 * day)})
	for i := 1; i <= 6; i++ {
		c := &taskmodel.Card{ID: "D" + string(rune('0'+i)), Title: "done"}
		if i == 1 {
			c.MilestoneID = "M1"
		}
		if i == 6 {
			c.Type = taskmodel.TypeBug
		}
		finished := now.Add(-day)
		h.Card(t, c,
			taskservicetest.Move{Status: taskmodel.StatusBacklog, At: finished.Add(-time.Duration(i)*day - day)},
			taskservicetest.Move{Status: taskmodel.StatusPlan, At: finished.Add(-time.Duration(i) * day)},
			taskservicetest.Move{Status: taskmodel.StatusDone, At: finished},
		)
	}
	h.Card(t, &taskmodel.Card{ID: "W1", MilestoneID: "M1", Title: "in flight", Docs: []string{"PRD001"}},
		taskservicetest.Move{Status: taskmodel.StatusPlan, At: now.Add(-10 * day)},
		taskservicetest.Move{Status: taskmodel.StatusWIP, At: now.Add(-8 * day)},
	)
	h.Card(t, &taskmodel.Card{ID: "W2", Title: "imported in review", Waiting: "dhira: approve"},
		taskservicetest.Move{Status: taskmodel.StatusReview, At: now.Add(-2 * day), Imported: true},
	)
	h.Card(t, &taskmodel.Card{ID: "B1", Title: "blocked", Type: taskmodel.TypeSecurityDebt, Deps: []string{"W1"}},
		taskservicetest.Move{Status: taskmodel.StatusBacklog, At: now.Add(-day)},
	)
	h.Card(t, &taskmodel.Card{ID: "UND", Title: "imported done without a date"},
		taskservicetest.Move{Status: taskmodel.StatusDone, At: now, Imported: true, Note: taskmodel.NoteImportedUndated},
	)
	h.Card(t, &taskmodel.Card{ID: "OLD", Title: "old", Folder: "tasks/missing"},
		taskservicetest.Move{Status: taskmodel.StatusDone, At: now.Add(-200 * day), Imported: true},
	)
	return h, insightservice.New(h.Tasks, h.Docs)
}

func TestOverview(t *testing.T) {
	_, svc := fixture(t)
	o, err := svc.Overview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ids := func(cards []*taskmodel.Card) []string {
		var out []string
		for _, c := range cards {
			out = append(out, c.ID)
		}
		return out
	}
	weekTotal := 0
	for _, w := range o.Throughput {
		weekTotal += w.Count
	}
	inFlight := map[string][2]bool{}
	for _, a := range o.InFlight {
		inFlight[a.Card.ID] = [2]bool{a.Known, a.Flagged}
	}
	var stale []string
	for _, a := range o.Stale {
		stale = append(stale, a.Card.ID)
	}
	tests := []struct {
		name     string
		got      any
		expected any
	}{
		{name: "cards, milestones excluded", got: o.Cards, expected: 11},
		{name: "a Done card imported without a date is not finished in a week", got: o.UndatedDone, expected: 1},
		{name: "WIP: Plan, WIP, QA, Review", got: o.WIP, expected: 2},
		{name: "cycle time sample", got: o.Cycle.N, expected: 6},
		{name: "cycle time p50 (nearest rank 3 of 6)", got: o.Cycle.P50, expected: 3 * day},
		{name: "cycle time p85 (nearest rank 6 of 6)", got: o.Cycle.P85, expected: 6 * day},
		{name: "cycle time per type: task and bug", got: len(o.CycleByType), expected: 2},
		{name: "throughput: six in the window, OLD outside", got: [2]int{weekTotal, o.Finished}, expected: [2]int{6, 6}},
		{name: "throughput weeks", got: len(o.Throughput), expected: insightmodel.ThroughputWeeks},
		{name: "age known and flagged over p85", got: inFlight["W1"], expected: [2]bool{true, true}},
		{name: "age unknown for a card imported already started", got: inFlight["W2"], expected: [2]bool{false, false}},
		{name: "stale: no event for over 7 days", got: stale, expected: []string{"W1"}},
		{name: "waiting on Dhira", got: ids(o.Waiting), expected: []string{"W2"}},
		{name: "in review", got: ids(o.InReview), expected: []string{"W2"}},
		{name: "blocked by an unfinished dependency", got: len(o.Blocked) == 1 && o.Blocked[0].Card.ID == "B1" && reflect.DeepEqual(o.Blocked[0].By, []string{"W1"}), expected: true},
		{name: "open cards and debt among them", got: [2]int{o.OpenTotal, o.OpenDebt}, expected: [2]int{3, 1}},
		{name: "milestone progress", got: [2]int{o.Milestones[0].Done, o.Milestones[0].Total}, expected: [2]int{1, 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.got, tt.expected) {
				t.Errorf("got %v, want %v", tt.got, tt.expected)
			}
		})
	}
}

func TestDocs(t *testing.T) {
	h, svc := fixture(t)
	path := filepath.Join(h.Vault, "docs/trd/TRD001.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("---\nid: TRD001\ntype: trd\ntitle: T\nstatus: draft\nupdated: 2026-11-20\nserves: [PRD001]\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Tasks.Update(context.Background(), &taskmodel.UpdateRequest{ID: "D1", Docs: &[]string{"TRD001"}, Actor: "dita"}); err != nil {
		t.Fatal(err)
	}
	o, err := svc.Docs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][3]int{}
	gaps := map[string][]string{}
	for _, p := range o.Docs {
		got[p.Doc.ID] = [3]int{p.Direct, p.Done, p.Total}
		gaps[p.Doc.ID] = p.Gaps
	}
	tests := []struct {
		name     string
		got      any
		expected any
	}{
		{name: "BRD counts its chain: W1 through PRD001, D1 through TRD001", got: got["BRD001"], expected: [3]int{0, 1, 2}},
		{name: "PRD direct W1 plus D1 below", got: got["PRD001"], expected: [3]int{1, 1, 2}},
		{name: "TRD direct D1, done", got: got["TRD001"], expected: [3]int{1, 1, 1}},
		{name: "a draft TRD whose cards are all Done", got: gaps["TRD001"], expected: []string{"still a draft while all its cards are Done"}},
		{name: "a BRD serving no OKR", got: gaps["BRD001"], expected: []string{"serves no OKR"}},
		{name: "counts by type and status", got: o.Counts["trd"]["draft"], expected: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.got, tt.expected) {
				t.Errorf("got %v, want %v", tt.got, tt.expected)
			}
		})
	}
}

func TestCheck(t *testing.T) {
	h, svc := fixture(t)
	if err := os.WriteFile(filepath.Join(h.Vault, "docs/bad.md"), []byte("---\ntype: prd\nid: x y\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	check, err := svc.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range check.Problems {
		got = append(got, p.Subject+": "+p.Message)
	}
	expected := []string{
		"OLD: folder tasks/missing does not exist",
		"docs/bad.md: invalid document: document ID \"x y\" (like PRD002 or OKR-2026Q4)",
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("Check() = %q, want %q", got, expected)
	}
}
