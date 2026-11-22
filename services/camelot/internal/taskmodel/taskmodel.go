// Package taskmodel holds the card domain: cards, their statuses, types
// and history events, the service requests and responses, and the
// domain's errors.
package taskmodel

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Status is where a card sits on the board.
type Status string

// The board's statuses, in column order.
const (
	StatusBacklog Status = "backlog"
	StatusTodo    Status = "todo"
	StatusPlan    Status = "plan"
	StatusWIP     Status = "wip"
	StatusQA      Status = "qa"
	StatusReview  Status = "review"
	StatusDone    Status = "done"
)

// Statuses lists every status in column order.
var Statuses = []Status{StatusBacklog, StatusTodo, StatusPlan, StatusWIP, StatusQA, StatusReview, StatusDone}

// statusLabels are the column titles.
var statusLabels = map[Status]string{
	StatusBacklog: "Backlog",
	StatusTodo:    "TODO",
	StatusPlan:    "Plan in Progress",
	StatusWIP:     "Work in Progress",
	StatusQA:      "QA in Progress",
	StatusReview:  "Review in Progress",
	StatusDone:    "Done",
}

// statusStages are the dev-loop stages each status covers.
var statusStages = map[Status]string{
	StatusBacklog: "captured",
	StatusTodo:    "plan approved, ready to start",
	StatusPlan:    "Plan",
	StatusWIP:     "Build, Quality Gate, Code Review",
	StatusQA:      "API Test, UI Test",
	StatusReview:  "Report, then Dhira's review",
	StatusDone:    "committed or closed",
}

// Label returns the status's column title.
func (s Status) Label() string { return statusLabels[s] }

// Stage returns the dev-loop stages the status covers.
func (s Status) Stage() string { return statusStages[s] }

// Started reports whether work on a card in this status has begun and not
// finished: Plan, WIP, QA or Review.
func (s Status) Started() bool {
	return s == StatusPlan || s == StatusWIP || s == StatusQA || s == StatusReview
}

// ParseStatus reads a status by its key ("wip") or its label ("Work in
// Progress"), case-insensitively.
func ParseStatus(raw string) (Status, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	for _, s := range Statuses {
		if v == string(s) || v == strings.ToLower(s.Label()) {
			return s, nil
		}
	}
	return "", fmt.Errorf("%w: %q (allowed: %s)", ErrInvalidStatus, raw, joinKeys(Statuses))
}

// Type is what kind of work a card is.
type Type string

// The card types.
const (
	TypeMilestone    Type = "milestone"
	TypeFeature      Type = "feature"
	TypeTask         Type = "task"
	TypeBug          Type = "bug"
	TypeResearch     Type = "research"
	TypeTechDebt     Type = "tech-debt"
	TypeFeatureDebt  Type = "feature-debt"
	TypeProductDebt  Type = "product-debt"
	TypeSecurityDebt Type = "security-debt"
)

// Types lists every type.
var Types = []Type{TypeMilestone, TypeFeature, TypeTask, TypeBug, TypeResearch, TypeTechDebt, TypeFeatureDebt, TypeProductDebt, TypeSecurityDebt}

// typeMeanings say what each type means.
var typeMeanings = map[Type]string{
	TypeMilestone:    "parent of cards (Jira's epic)",
	TypeFeature:      "new behaviour a user or merchant sees",
	TypeTask:         "work with no user-visible change: docs, setup, chores",
	TypeBug:          "behaviour differs from its spec or PRD",
	TypeResearch:     "the output is a decision document, not code",
	TypeTechDebt:     "code, test or operational shortcut users don't see",
	TypeFeatureDebt:  "functionality deliberately deferred",
	TypeProductDebt:  "a shortcut users feel: UX, copy, flows",
	TypeSecurityDebt: "a deferred security, privacy or compliance control",
}

// Meaning returns what the type means.
func (t Type) Meaning() string { return typeMeanings[t] }

// IsDebt reports whether the type is one of the four debt kinds.
func (t Type) IsDebt() bool {
	return t == TypeTechDebt || t == TypeFeatureDebt || t == TypeProductDebt || t == TypeSecurityDebt
}

// ParseType reads a type by its key, case-insensitively.
func ParseType(raw string) (Type, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	for _, t := range Types {
		if v == string(t) {
			return t, nil
		}
	}
	return "", fmt.Errorf("%w: %q (allowed: %s)", ErrInvalidType, raw, joinKeys(Types))
}

// idPattern is a card ID: a prefix of capitals and digits, then optionally
// a dash and letters, digits or +.
var idPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]*(-[A-Za-z0-9+]+)?$`)

// prefixPattern is the prefix for a generated ID.
var prefixPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]*$`)

// MaxIDLength bounds a card ID.
const MaxIDLength = 40

// MaxTitleLength bounds a title, in bytes.
const MaxTitleLength = 300

// MaxDescriptionLength bounds a description, in bytes.
const MaxDescriptionLength = 200_000

// ParseCardID reads a card ID such as "WIG-081" or "M3".
func ParseCardID(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if len(v) > MaxIDLength || !idPattern.MatchString(v) {
		return "", fmt.Errorf("%w: %q (like WIG-081, DITA-004 or M3)", ErrInvalidID, raw)
	}
	return v, nil
}

// ParsePrefix reads the prefix of a generated ID such as "DITA".
func ParsePrefix(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if len(v) > 12 || !prefixPattern.MatchString(v) {
		return "", fmt.Errorf("%w: prefix %q (capitals and digits, like DITA)", ErrInvalidID, raw)
	}
	return v, nil
}

// ParseTitle reads a title: trimmed, one line, 1 to MaxTitleLength bytes.
func ParseTitle(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" || len(v) > MaxTitleLength || strings.ContainsAny(v, "\r\n") {
		return "", fmt.Errorf("%w: a title is one line of 1 to %d bytes", ErrInvalidField, MaxTitleLength)
	}
	return v, nil
}

// ParseOneLine reads an optional one-line field (assignee, waiting).
func ParseOneLine(field, raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if len(v) > MaxTitleLength || strings.ContainsAny(v, "\r\n") {
		return "", fmt.Errorf("%w: %s is one line of at most %d bytes", ErrInvalidField, field, MaxTitleLength)
	}
	return v, nil
}

// ParseDescription reads a description: Markdown up to MaxDescriptionLength bytes.
func ParseDescription(raw string) (string, error) {
	if len(raw) > MaxDescriptionLength {
		return "", fmt.Errorf("%w: a description is at most %d bytes", ErrInvalidField, MaxDescriptionLength)
	}
	return strings.TrimSpace(strings.ReplaceAll(raw, "\r\n", "\n")), nil
}

// ParseIDList reads a list of card or document IDs: comma separated,
// deduplicated, in the given order.
func ParseIDList(raw []string, parse func(string) (string, error)) ([]string, error) {
	out := []string{}
	for _, item := range raw {
		for part := range strings.SplitSeq(item, ",") {
			if strings.TrimSpace(part) == "" {
				continue
			}
			id, err := parse(part)
			if err != nil {
				return nil, err
			}
			if !slices.Contains(out, id) {
				out = append(out, id)
			}
		}
	}
	return out, nil
}

// Card is one unit of work.
type Card struct {
	ID          string    `json:"id"`
	MilestoneID string    `json:"milestone_id"`
	Title       string    `json:"title"`
	Type        Type      `json:"type"`
	Status      Status    `json:"status"`
	Assignee    string    `json:"assignee"`
	Waiting     string    `json:"waiting"`
	Options     []string  `json:"waiting_options"`
	Folder      string    `json:"folder"`
	Description string    `json:"description,omitempty"`
	Deps        []string  `json:"deps"`
	Docs        []string  `json:"docs"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Event fields, the card attribute an event records.
const (
	FieldCreated     = "created"
	FieldImported    = "imported"
	FieldDeleted     = "deleted"
	FieldStatus      = "status"
	FieldTitle       = "title"
	FieldType        = "type"
	FieldMilestone   = "milestone"
	FieldAssignee    = "assignee"
	FieldWaiting     = "waiting"
	FieldOptions     = "waiting_options"
	FieldDecision    = "decision"
	FieldFolder      = "folder"
	FieldDescription = "description"
	FieldDeps        = "deps"
	FieldDocs        = "docs"
)

// NoteImportedUndated marks the import event of a Done card whose board
// line had no done date: it counts as Done, never as finished in a week.
const NoteImportedUndated = "imported from TASK.md without a done date"

// Event is one recorded change to a card; events are never edited.
type Event struct {
	ID        int64     `json:"id"`
	CardID    string    `json:"card_id"`
	Actor     string    `json:"actor"`
	Field     string    `json:"field"`
	FromValue string    `json:"from_value"`
	ToValue   string    `json:"to_value"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

// NewEvent returns the event of actor changing field from one value to another.
func NewEvent(cardID, actor, field, from, to, note string, at time.Time) *Event {
	return &Event{
		CardID:    cardID,
		Actor:     actor,
		Field:     field,
		FromValue: from,
		ToValue:   to,
		Note:      note,
		CreatedAt: at,
	}
}

// CardFilter narrows a card list; zero fields match everything.
type CardFilter struct {
	Statuses    []Status
	Types       []Type
	MilestoneID string
	Assignee    string
	DocID       string
	Waiting     bool
	Query       string
}

// EventFilter narrows an event list; zero fields match everything.
type EventFilter struct {
	CardID  string
	Since   time.Time
	AfterID int64
	Limit   int
}

// CreateRequest creates a card. ID or Prefix: with only Prefix the next
// free number is taken (DITA → DITA-011). Imported records an "imported"
// event at CreatedAt instead of "created" now.
type CreateRequest struct {
	ID          string
	Prefix      string
	MilestoneID string
	Title       string
	Type        string
	Status      string
	Assignee    string
	Waiting     string
	Options     []string
	Folder      string
	Description string
	Deps        []string
	Docs        []string
	Actor       string
	Note        string
	Imported    bool
	CreatedAt   time.Time
}

// UpdateRequest changes the fields that are set (nil = unchanged) on one
// card. UpdatedAt, when set, must equal the card's, else ErrStale.
type UpdateRequest struct {
	ID          string
	MilestoneID *string
	Title       *string
	Type        *string
	Status      *string
	Assignee    *string
	Waiting     *string
	Options     *[]string
	Folder      *string
	Description *string
	Deps        *[]string
	Docs        *[]string
	Actor       string
	Note        string
	UpdatedAt   time.Time
}

// AnswerRequest answers the question waiting on a card: the answer is
// recorded as a decision event and the question and its options clear.
type AnswerRequest struct {
	ID        string
	Answer    string
	Actor     string
	UpdatedAt time.Time
}

// MaxOptions bounds the answer options of a question.
const MaxOptions = 6

// ParseOptions reads a question's answer options: up to MaxOptions one-line, distinct, non-empty items.
func ParseOptions(raw []string) ([]string, error) {
	out := []string{}
	for _, o := range raw {
		v, err := ParseOneLine("an option", o)
		if err != nil {
			return nil, err
		}
		if v != "" && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	if len(out) > MaxOptions {
		return nil, fmt.Errorf("%w: at most %d options", ErrInvalidField, MaxOptions)
	}
	return out, nil
}

// DeleteRequest deletes one card; its events stay.
type DeleteRequest struct {
	ID    string
	Actor string
	Note  string
}

// GetRequest reads one card.
type GetRequest struct {
	ID string
}

// ListRequest lists cards.
type ListRequest struct {
	Filter CardFilter
}

// FilesRequest lists the Markdown files of a card's folder.
type FilesRequest struct {
	ID string
}

// FileRequest reads one Markdown file of a card's folder.
type FileRequest struct {
	ID   string
	Name string
}

// FileResponse is a card folder file with its card.
type FileResponse struct {
	Card    *Card
	Name    string
	Content []byte
}

// DepState is a dependency with its status, for a card's detail.
type DepState struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status Status `json:"status"`
	Found  bool   `json:"found"`
}

// GetResponse is a card with its dependencies' states, its dependents and
// its history, newest first.
type GetResponse struct {
	Card       *Card      `json:"card"`
	Deps       []DepState `json:"deps"`
	Dependents []*Card    `json:"dependents"`
	Children   []*Card    `json:"children"`
	Events     []*Event   `json:"events"`
}

// Blocked reports whether any dependency is not Done.
func (r *GetResponse) Blocked() bool {
	for _, d := range r.Deps {
		if d.Status != StatusDone {
			return true
		}
	}
	return false
}

// The domain's errors.
var (
	ErrNotFound        = errors.New("card not found")
	ErrExists          = errors.New("card already exists")
	ErrStale           = errors.New("card changed since it was read; reload and retry")
	ErrInvalidID       = errors.New("invalid card ID")
	ErrInvalidStatus   = errors.New("invalid status")
	ErrInvalidType     = errors.New("invalid type")
	ErrInvalidField    = errors.New("invalid field")
	ErrInvalidLink     = errors.New("invalid link")
	ErrNothingToChange = errors.New("nothing to change")
	ErrFolderMissing   = errors.New("card folder does not exist")
	ErrNothingWaiting  = errors.New("no question waits on this card")
	ErrFileNotFound    = errors.New("no such file in the card folder")
)

// joinKeys lists the keys of a string-typed enum.
func joinKeys[T ~string](values []T) string {
	keys := make([]string, len(values))
	for i, v := range values {
		keys[i] = string(v)
	}
	return strings.Join(keys, ", ")
}
