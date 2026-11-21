// Package docmodel holds the document domain: business, product and
// technical requirement documents, OKRs and KPIs, kept as Markdown files
// with frontmatter, their chain and the domain's errors.
package docmodel

import (
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
	ID      string   `json:"id"`
	Type    Type     `json:"type"`
	Title   string   `json:"title"`
	Status  Status   `json:"status"`
	Owner   string   `json:"owner"`
	Updated string   `json:"updated"`
	Serves  []string `json:"serves"`
	Path    string   `json:"path"`
	Body    []byte   `json:"-"`
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
}

// The domain's errors.
var (
	ErrNotFound = errors.New("document not found")
	ErrInvalid  = errors.New("invalid document")
)
