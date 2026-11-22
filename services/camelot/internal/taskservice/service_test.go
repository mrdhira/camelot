package taskservice_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/mrdhira/camelot/services/camelot/internal/taskmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/taskservice/taskservicetest"
)

func ptr[T any](v T) *T { return &v }

func TestCreate(t *testing.T) {
	type expected struct {
		id       string
		status   taskmodel.Status
		cardType taskmodel.Type
		field    string
		err      error
	}
	tests := []struct {
		name     string
		input    *taskmodel.CreateRequest
		expected expected
	}{
		{
			name:     "a card with an ID, links and defaults",
			input:    &taskmodel.CreateRequest{ID: "DITA-100", Title: "Build it", MilestoneID: "M1", Deps: []string{"DITA-001"}, Docs: []string{"PRD001"}, Actor: "dita"},
			expected: expected{id: "DITA-100", status: taskmodel.StatusBacklog, cardType: taskmodel.TypeTask, field: taskmodel.FieldCreated},
		},
		{
			name:     "the next number after the highest for a prefix",
			input:    &taskmodel.CreateRequest{Prefix: "DITA", Title: "Next", Type: "bug", Status: "Work in Progress", Actor: "dita"},
			expected: expected{id: "DITA-002", status: taskmodel.StatusWIP, cardType: taskmodel.TypeBug, field: taskmodel.FieldCreated},
		},
		{
			name:     "an imported card records an imported event",
			input:    &taskmodel.CreateRequest{ID: "WIG-001", Title: "Old", Status: "done", Imported: true, CreatedAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), Actor: "import"},
			expected: expected{id: "WIG-001", status: taskmodel.StatusDone, cardType: taskmodel.TypeTask, field: taskmodel.FieldImported},
		},
		{name: "no actor", input: &taskmodel.CreateRequest{ID: "X-1", Title: "t"}, expected: expected{err: taskmodel.ErrInvalidField}},
		{name: "neither ID nor prefix", input: &taskmodel.CreateRequest{Title: "t", Actor: "dita"}, expected: expected{err: taskmodel.ErrInvalidID}},
		{name: "a malformed ID", input: &taskmodel.CreateRequest{ID: "wig 1", Title: "t", Actor: "dita"}, expected: expected{err: taskmodel.ErrInvalidID}},
		{name: "an existing ID", input: &taskmodel.CreateRequest{ID: "DITA-001", Title: "t", Actor: "dita"}, expected: expected{err: taskmodel.ErrExists}},
		{name: "an empty title", input: &taskmodel.CreateRequest{ID: "X-1", Title: " ", Actor: "dita"}, expected: expected{err: taskmodel.ErrInvalidField}},
		{name: "an unknown type", input: &taskmodel.CreateRequest{ID: "X-1", Title: "t", Type: "story", Actor: "dita"}, expected: expected{err: taskmodel.ErrInvalidType}},
		{name: "an unknown status", input: &taskmodel.CreateRequest{ID: "X-1", Title: "t", Status: "blocked", Actor: "dita"}, expected: expected{err: taskmodel.ErrInvalidStatus}},
		{name: "a folder outside tasks/", input: &taskmodel.CreateRequest{ID: "X-1", Title: "t", Folder: "tasks/../etc", Actor: "dita"}, expected: expected{err: taskmodel.ErrInvalidField}},
		{name: "a milestone that is not a milestone", input: &taskmodel.CreateRequest{ID: "X-1", Title: "t", MilestoneID: "DITA-001", Actor: "dita"}, expected: expected{err: taskmodel.ErrInvalidLink}},
		{name: "a missing dependency", input: &taskmodel.CreateRequest{ID: "X-1", Title: "t", Deps: []string{"NOPE-1"}, Actor: "dita"}, expected: expected{err: taskmodel.ErrInvalidLink}},
		{name: "a missing document", input: &taskmodel.CreateRequest{ID: "X-1", Title: "t", Docs: []string{"PRD999"}, Actor: "dita"}, expected: expected{err: taskmodel.ErrInvalidLink}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := taskservicetest.New(t)
			ctx := context.Background()
			h.Seed(t)
			card, err := h.Tasks.Create(ctx, tt.input)
			if tt.expected.err != nil {
				if !errors.Is(err, tt.expected.err) {
					t.Fatalf("Create() error = %v, want %v", err, tt.expected.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			if card.ID != tt.expected.id || card.Status != tt.expected.status || card.Type != tt.expected.cardType {
				t.Errorf("Create() = %s %s %s, want %s %s %s", card.ID, card.Status, card.Type, tt.expected.id, tt.expected.status, tt.expected.cardType)
			}
			got, err := h.Tasks.Get(ctx, &taskmodel.GetRequest{ID: card.ID})
			if err != nil {
				t.Fatalf("Get() error = %v", err)
			}
			if len(got.Events) != 1 || got.Events[0].Field != tt.expected.field || got.Events[0].ToValue != string(tt.expected.status) {
				t.Errorf("events = %+v, want one %s event to %s", got.Events, tt.expected.field, tt.expected.status)
			}
			if !reflect.DeepEqual(got.Card.Deps, card.Deps) || !reflect.DeepEqual(got.Card.Docs, card.Docs) {
				t.Errorf("stored links = %v %v, want %v %v", got.Card.Deps, got.Card.Docs, card.Deps, card.Docs)
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	type expected struct {
		fields []string
		status taskmodel.Status
		err    error
	}
	tests := []struct {
		name     string
		input    *taskmodel.UpdateRequest
		expected expected
	}{
		{
			name:     "a move records one status event",
			input:    &taskmodel.UpdateRequest{ID: "DITA-001", Status: ptr("wip"), Actor: "go-engineer", Note: "build started"},
			expected: expected{fields: []string{taskmodel.FieldStatus}, status: taskmodel.StatusWIP},
		},
		{
			name:     "several fields record one event each; an unchanged field none",
			input:    &taskmodel.UpdateRequest{ID: "DITA-001", Title: ptr("Seed card"), Waiting: ptr("dhira: pick A or B"), Docs: &[]string{"PRD001"}, Description: ptr("new text"), Actor: "dita"},
			expected: expected{fields: []string{taskmodel.FieldWaiting, taskmodel.FieldDescription, taskmodel.FieldDocs}, status: taskmodel.StatusBacklog},
		},
		{name: "a milestone with cards cannot stop being one", input: &taskmodel.UpdateRequest{ID: "M1", Type: ptr("task"), Actor: "dita"}, expected: expected{err: taskmodel.ErrInvalidLink}},
		{name: "nothing changes", input: &taskmodel.UpdateRequest{ID: "DITA-001", Title: ptr("Seed card"), Actor: "dita"}, expected: expected{err: taskmodel.ErrNothingToChange}},
		{name: "a missing card", input: &taskmodel.UpdateRequest{ID: "NOPE-1", Status: ptr("wip"), Actor: "dita"}, expected: expected{err: taskmodel.ErrNotFound}},
		{name: "a stale read", input: &taskmodel.UpdateRequest{ID: "DITA-001", Status: ptr("wip"), Actor: "dita", UpdatedAt: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)}, expected: expected{err: taskmodel.ErrStale}},
		{name: "an unknown status", input: &taskmodel.UpdateRequest{ID: "DITA-001", Status: ptr("doing"), Actor: "dita"}, expected: expected{err: taskmodel.ErrInvalidStatus}},
		{name: "a dependency cycle", input: &taskmodel.UpdateRequest{ID: "DITA-001", Deps: &[]string{"DITA-005"}, Actor: "dita"}, expected: expected{err: taskmodel.ErrInvalidLink}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := taskservicetest.New(t)
			ctx := context.Background()
			h.Seed(t)
			if _, err := h.Tasks.Create(ctx, &taskmodel.CreateRequest{ID: "DITA-005", Title: "Depends on seed", Deps: []string{"DITA-001"}, Actor: "dita"}); err != nil {
				t.Fatal(err)
			}
			card, err := h.Tasks.Update(ctx, tt.input)
			if tt.expected.err != nil {
				if !errors.Is(err, tt.expected.err) {
					t.Fatalf("Update() error = %v, want %v", err, tt.expected.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Update() error = %v", err)
			}
			if card.Status != tt.expected.status {
				t.Errorf("Status = %s, want %s", card.Status, tt.expected.status)
			}
			got, err := h.Tasks.Get(ctx, &taskmodel.GetRequest{ID: tt.input.ID})
			if err != nil {
				t.Fatal(err)
			}
			var fields []string
			for _, e := range got.Events {
				if e.Field != taskmodel.FieldCreated {
					fields = append([]string{e.Field}, fields...)
				}
			}
			if !reflect.DeepEqual(fields, tt.expected.fields) {
				t.Errorf("event fields = %v, want %v", fields, tt.expected.fields)
			}
		})
	}
}

func TestUpdateStaleAfterAnotherWrite(t *testing.T) {
	h := taskservicetest.New(t)
	ctx := context.Background()
	h.Seed(t)
	read, err := h.Tasks.Get(ctx, &taskmodel.GetRequest{ID: "DITA-001"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.Tasks.Update(ctx, &taskmodel.UpdateRequest{ID: "DITA-001", Status: ptr("todo"), Actor: "dita", UpdatedAt: read.Card.UpdatedAt}); err != nil {
		t.Fatalf("first Update() with a fresh read: %v", err)
	}
	_, err = h.Tasks.Update(ctx, &taskmodel.UpdateRequest{ID: "DITA-001", Status: ptr("plan"), Actor: "dhira", UpdatedAt: read.Card.UpdatedAt})
	if !errors.Is(err, taskmodel.ErrStale) {
		t.Fatalf("second Update() with the old read: error = %v, want ErrStale", err)
	}
}

func TestDelete(t *testing.T) {
	tests := []struct {
		name     string
		input    *taskmodel.DeleteRequest
		expected error
	}{
		{name: "a card nothing depends on", input: &taskmodel.DeleteRequest{ID: "DITA-005", Actor: "dhira"}},
		{name: "a card another depends on", input: &taskmodel.DeleteRequest{ID: "DITA-001", Actor: "dhira"}, expected: taskmodel.ErrInvalidLink},
		{name: "a milestone with cards", input: &taskmodel.DeleteRequest{ID: "M1", Actor: "dhira"}, expected: taskmodel.ErrInvalidLink},
		{name: "a missing card", input: &taskmodel.DeleteRequest{ID: "NOPE-1", Actor: "dhira"}, expected: taskmodel.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := taskservicetest.New(t)
			ctx := context.Background()
			h.Seed(t)
			if _, err := h.Tasks.Create(ctx, &taskmodel.CreateRequest{ID: "DITA-005", Title: "Depends on seed", MilestoneID: "M1", Deps: []string{"DITA-001"}, Actor: "dita"}); err != nil {
				t.Fatal(err)
			}
			err := h.Tasks.Delete(ctx, tt.input)
			if !errors.Is(err, tt.expected) {
				t.Fatalf("Delete() error = %v, want %v", err, tt.expected)
			}
			if tt.expected != nil {
				return
			}
			if _, err := h.Tasks.Get(ctx, &taskmodel.GetRequest{ID: tt.input.ID}); !errors.Is(err, taskmodel.ErrNotFound) {
				t.Errorf("Get() after delete error = %v, want ErrNotFound", err)
			}
			events, err := h.Tasks.Events(ctx, &taskmodel.EventFilter{CardID: tt.input.ID})
			if err != nil || len(events) == 0 || events[0].Field != taskmodel.FieldDeleted {
				t.Errorf("events after delete = %+v, %v; want a deleted event first", events, err)
			}
			if _, err := h.Tasks.Create(ctx, &taskmodel.CreateRequest{ID: tt.input.ID, Title: "again", Actor: "dita"}); !errors.Is(err, taskmodel.ErrExists) {
				t.Errorf("re-create a deleted ID error = %v, want ErrExists", err)
			}
		})
	}
}

func TestList(t *testing.T) {
	h := taskservicetest.New(t)
	ctx := context.Background()
	h.Seed(t)
	if _, err := h.Tasks.Create(ctx, &taskmodel.CreateRequest{ID: "DITA-005", Title: "Waiting card", Status: "review", Waiting: "dhira: approve", Docs: []string{"PRD001"}, Actor: "dita"}); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		input    taskmodel.CardFilter
		expected []string
	}{
		{name: "everything, by ID", input: taskmodel.CardFilter{}, expected: []string{"DITA-001", "DITA-005", "M1"}},
		{name: "by status", input: taskmodel.CardFilter{Statuses: []taskmodel.Status{taskmodel.StatusReview}}, expected: []string{"DITA-005"}},
		{name: "by type", input: taskmodel.CardFilter{Types: []taskmodel.Type{taskmodel.TypeMilestone}}, expected: []string{"M1"}},
		{name: "waiting on Dhira", input: taskmodel.CardFilter{Waiting: true}, expected: []string{"DITA-005"}},
		{name: "linked to a document", input: taskmodel.CardFilter{DocID: "PRD001"}, expected: []string{"DITA-005"}},
		{name: "a query on the title, wildcards literal", input: taskmodel.CardFilter{Query: "seed"}, expected: []string{"DITA-001"}},
		{name: "a query matching nothing", input: taskmodel.CardFilter{Query: "%"}, expected: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cards, err := h.Tasks.List(ctx, &taskmodel.ListRequest{Filter: tt.input})
			if err != nil {
				t.Fatal(err)
			}
			var ids []string
			for _, c := range cards {
				ids = append(ids, c.ID)
			}
			if !reflect.DeepEqual(ids, tt.expected) {
				t.Errorf("List() = %v, want %v", ids, tt.expected)
			}
		})
	}
}

func TestAnswer(t *testing.T) {
	type expected struct {
		err      error
		decision string
	}
	tests := []struct {
		name     string
		waiting  bool
		input    *taskmodel.AnswerRequest
		expected expected
	}{
		{name: "an answer records the decision and clears the question and its options", waiting: true, input: &taskmodel.AnswerRequest{ID: "DITA-001", Answer: "B: cheaper", Actor: "dhira"}, expected: expected{decision: "Pick A or B? → B: cheaper"}},
		{name: "nothing waits", input: &taskmodel.AnswerRequest{ID: "DITA-001", Answer: "B", Actor: "dhira"}, expected: expected{err: taskmodel.ErrNothingWaiting}},
		{name: "an empty answer", waiting: true, input: &taskmodel.AnswerRequest{ID: "DITA-001", Answer: "  ", Actor: "dhira"}, expected: expected{err: taskmodel.ErrInvalidField}},
		{name: "a stale read", waiting: true, input: &taskmodel.AnswerRequest{ID: "DITA-001", Answer: "B", Actor: "dhira", UpdatedAt: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)}, expected: expected{err: taskmodel.ErrStale}},
		{name: "a missing card", input: &taskmodel.AnswerRequest{ID: "NOPE-1", Answer: "B", Actor: "dhira"}, expected: expected{err: taskmodel.ErrNotFound}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := taskservicetest.New(t)
			ctx := context.Background()
			h.Seed(t)
			if tt.waiting {
				if _, err := h.Tasks.Update(ctx, &taskmodel.UpdateRequest{ID: "DITA-001", Waiting: ptr("Pick A or B?"), Options: &[]string{"A", "B: cheaper"}, Actor: "dita"}); err != nil {
					t.Fatal(err)
				}
			}
			card, err := h.Tasks.Answer(ctx, tt.input)
			if !errors.Is(err, tt.expected.err) {
				t.Fatalf("Answer() error = %v, want %v", err, tt.expected.err)
			}
			if tt.expected.err != nil {
				return
			}
			if card.Waiting != "" || len(card.Options) != 0 {
				t.Errorf("after the answer waiting = %q, options = %v; want both cleared", card.Waiting, card.Options)
			}
			got, err := h.Tasks.Get(ctx, &taskmodel.GetRequest{ID: "DITA-001"})
			if err != nil {
				t.Fatal(err)
			}
			e := got.Events[0]
			if e.Field != taskmodel.FieldDecision || e.FromValue+" → "+e.ToValue != tt.expected.decision || e.Actor != "dhira" {
				t.Errorf("newest event = %+v, want the decision %q by dhira", e, tt.expected.decision)
			}
		})
	}
}

func TestOptions(t *testing.T) {
	tests := []struct {
		name     string
		input    *taskmodel.UpdateRequest
		expected []string
		err      error
	}{
		{name: "options with a question, deduplicated", input: &taskmodel.UpdateRequest{ID: "DITA-001", Waiting: ptr("Which?"), Options: &[]string{"A", "B", "A"}, Actor: "dita"}, expected: []string{"A", "B"}},
		{name: "a new question without options drops the old question's options", input: &taskmodel.UpdateRequest{ID: "DITA-001", Waiting: ptr("Which database?"), Actor: "dita"}, expected: []string{}},
		{name: "options without a question", input: &taskmodel.UpdateRequest{ID: "DITA-001", Waiting: ptr(""), Options: &[]string{"A"}, Actor: "dita"}, err: taskmodel.ErrInvalidField},
		{name: "too many options", input: &taskmodel.UpdateRequest{ID: "DITA-001", Waiting: ptr("Which?"), Options: &[]string{"1", "2", "3", "4", "5", "6", "7"}, Actor: "dita"}, err: taskmodel.ErrInvalidField},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := taskservicetest.New(t)
			h.Seed(t)
			if _, err := h.Tasks.Update(context.Background(), &taskmodel.UpdateRequest{ID: "DITA-001", Waiting: ptr("Pick A or B?"), Options: &[]string{"A", "B"}, Actor: "dita"}); err != nil {
				t.Fatal(err)
			}
			card, err := h.Tasks.Update(context.Background(), tt.input)
			if !errors.Is(err, tt.err) {
				t.Fatalf("Update() error = %v, want %v", err, tt.err)
			}
			if tt.err == nil && len(card.Options)+len(tt.expected) > 0 && !reflect.DeepEqual(card.Options, tt.expected) {
				t.Errorf("Options = %v, want %v", card.Options, tt.expected)
			}
		})
	}
}
