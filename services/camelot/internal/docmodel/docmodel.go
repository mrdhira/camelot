// Package docmodel holds the document domain: business, product and
// technical requirement documents, OKRs and KPIs, kept as Markdown files
// with frontmatter, their chain and the domain's errors.
package docmodel

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Type is a document's kind.
type Type string

// The document types, top of the chain first.
const (
	TypeOKR Type = "okr"
	TypeKPI Type = "kpi"
	TypeBRD Type = "brd"
	TypePRD Type = "prd"
	TypeTRD Type = "trd"
)

// Types lists every type, top of the chain first.
var Types = []Type{TypeOKR, TypeKPI, TypeBRD, TypePRD, TypeTRD}

// typeLabels are the types' names.
var typeLabels = map[Type]string{
	TypeOKR: "OKR",
	TypeKPI: "KPI",
	TypeBRD: "Business Requirement",
	TypePRD: "Product Requirement",
	TypeTRD: "Technical Requirement",
}

// Label returns the type's name.
func (t Type) Label() string { return typeLabels[t] }

// Short returns the type's abbreviation (BRD, PRD…).
func (t Type) Short() string { return strings.ToUpper(string(t)) }

// Rank is the type's place in the chain: 0 for OKR, 4 for TRD.
func (t Type) Rank() int {
	for i, v := range Types {
		if v == t {
			return i
		}
	}
	return len(Types)
}

// ParseType reads a type by its key, case-insensitively.
func ParseType(raw string) (Type, error) {
	v := Type(strings.ToLower(strings.TrimSpace(raw)))
	for _, t := range Types {
		if v == t {
			return t, nil
		}
	}
	return "", fmt.Errorf("%w: type %q (allowed: okr, kpi, brd, prd, trd)", ErrInvalid, raw)
}

// Status is a document's review state.
type Status string

// The document statuses, in order.
const (
	StatusDraft      Status = "draft"
	StatusInReview   Status = "in-review"
	StatusApproved   Status = "approved"
	StatusSuperseded Status = "superseded"
)

// Statuses lists every status in order.
var Statuses = []Status{StatusDraft, StatusInReview, StatusApproved, StatusSuperseded}

// statusLabels are the statuses' names.
var statusLabels = map[Status]string{
	StatusDraft:      "Draft",
	StatusInReview:   "In review",
	StatusApproved:   "Approved",
	StatusSuperseded: "Superseded",
}

// Label returns the status's name.
func (s Status) Label() string { return statusLabels[s] }

// ParseStatus reads a status by its key or label, case-insensitively.
func ParseStatus(raw string) (Status, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	for _, s := range Statuses {
		if v == string(s) || v == strings.ToLower(s.Label()) {
			return s, nil
		}
	}
	return "", fmt.Errorf("%w: status %q (allowed: draft, in-review, approved, superseded)", ErrInvalid, raw)
}

// idPattern is a document ID such as PRD002, BRD-001 or OKR-2026Q4.
var idPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]*(-[A-Z0-9]+)*$`)

// ParseID reads a document ID.
func ParseID(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if len(v) > 40 || !idPattern.MatchString(v) {
		return "", fmt.Errorf("%w: document ID %q (like PRD002 or OKR-2026Q4)", ErrInvalid, raw)
	}
	return v, nil
}

// The frontmatter keys a document carries.
const (
	KeyID      = "id"
	KeyType    = "type"
	KeyTitle   = "title"
	KeyStatus  = "status"
	KeyOwner   = "owner"
	KeyUpdated = "updated"
	KeyServes  = "serves"
)

// DateLayout is the layout of the updated date.
const DateLayout = time.DateOnly

// Doc is one document file.
type Doc struct {
	ID         string    `json:"id"`
	Type       Type      `json:"type"`
	Title      string    `json:"title"`
	Status     Status    `json:"status"`
	Owner      string    `json:"owner"`
	Updated    string    `json:"updated"`
	Serves     []string  `json:"serves"`
	Path       string    `json:"path"`
	Hash       string    `json:"-"`
	ModifiedAt time.Time `json:"modified_at"`
	Body       []byte    `json:"-"`
}

// HashOf is a document content's hash, as snapshots record it.
func HashOf(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// The kinds of document change.
const (
	KindCreated = "created"
	KindEdited  = "edited"
	KindStatus  = "status"
	KindRemoved = "removed"
)

// ActorFile is who changed a document edited outside Camelot: unknown.
const ActorFile = "file"

// Snapshot is what Camelot last saw of a document file.
type Snapshot struct {
	DocID      string
	Path       string
	Title      string
	Type       Type
	Status     Status
	Hash       string
	ModifiedAt time.Time
	SyncedAt   time.Time
}

// NewSnapshot returns the snapshot of doc taken at now.
func NewSnapshot(doc *Doc, now time.Time) *Snapshot {
	return &Snapshot{
		DocID:      doc.ID,
		Path:       doc.Path,
		Title:      doc.Title,
		Type:       doc.Type,
		Status:     doc.Status,
		Hash:       doc.Hash,
		ModifiedAt: doc.ModifiedAt,
		SyncedAt:   now,
	}
}

// Change is one document's sync: its new snapshot (nil to remove it), the
// hash of the snapshot it was computed from ("" when there was none), and
// its events. A change whose base no longer matches is skipped: another
// process already recorded it.
type Change struct {
	DocID    string
	BaseHash string
	Snapshot *Snapshot
	Events   []*Event
}

// Event is one recorded change of a document.
type Event struct {
	ID        int64     `json:"id"`
	DocID     string    `json:"doc_id"`
	Actor     string    `json:"actor"`
	Kind      string    `json:"kind"`
	FromValue string    `json:"from_value"`
	ToValue   string    `json:"to_value"`
	CreatedAt time.Time `json:"created_at"`
}

// NewEvent returns the event of a document change.
func NewEvent(docID, actor, kind, from, to string, at time.Time) *Event {
	return &Event{
		DocID:     docID,
		Actor:     actor,
		Kind:      kind,
		FromValue: from,
		ToValue:   to,
		CreatedAt: at,
	}
}

// EventFilter narrows document events; zero fields match everything.
type EventFilter struct {
	DocID   string
	Since   time.Time
	AfterID int64
	Limit   int
}

// Problem is something wrong in a document file or its links.
type Problem struct {
	Path    string `json:"path"`
	DocID   string `json:"doc_id"`
	Message string `json:"message"`
}

// Chain is a document with what it serves (up to the top) and what serves it (down to the bottom).
type Chain struct {
	Doc  *Doc   `json:"doc"`
	Up   []*Doc `json:"up"`
	Down []*Doc `json:"down"`
}

// ListRequest lists documents.
type ListRequest struct {
	Types    []Type
	Statuses []Status
}

// ListResponse is the documents and the problems found reading them.
type ListResponse struct {
	Docs     []*Doc    `json:"docs"`
	Problems []Problem `json:"problems"`
}

// GetRequest reads one document.
type GetRequest struct {
	ID string
}

// SetStatusRequest changes one document's status and stamps its updated date.
type SetStatusRequest struct {
	ID     string
	Status string
	Actor  string
}

// The domain's errors.
var (
	ErrNotFound = errors.New("document not found")
	ErrInvalid  = errors.New("invalid document")
)
