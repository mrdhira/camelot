// Package commentmodel holds comments: one conversation per card and per
// document, separate from decisions. Comments are append-only; resolving
// one is another row pointing at it.
package commentmodel

import (
	"errors"
	"time"
)

// The kinds of comment target.
const (
	KindCard = "card"
	KindDoc  = "doc"
)

// MaxBody bounds a comment's text, in characters.
const MaxBody = 4000

// Owner is the browser's actor: a card whose newest comment is the owner's,
// still open, waits for an agent's reply.
const Owner = "dhira"

// Comment is one comment, or (ResolvesID set) the resolution of one.
type Comment struct {
	ID         int64      `json:"id"`
	Kind       string     `json:"kind"`
	TargetID   string     `json:"target_id"`
	Section    string     `json:"section,omitempty"`
	Body       string     `json:"body"`
	Actor      string     `json:"actor"`
	CreatedAt  time.Time  `json:"created_at"`
	ResolvesID int64      `json:"resolves_id,omitempty"`
	ResolvedBy string     `json:"resolved_by,omitempty"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
	// ResolvedNote is the resolution's note, when it has one.
	ResolvedNote string `json:"resolved_note,omitempty"`
}

// Open reports whether c is a comment nobody resolved.
func (c *Comment) Open() bool { return c.ResolvesID == 0 && c.ResolvedAt == nil }

// Thread is a target's comments, oldest first, with what they add up to.
type Thread struct {
	Comments []*Comment `json:"comments"`
	Open     int        `json:"open"`
	// AwaitsReply is true when the newest comment is the owner's and open.
	AwaitsReply bool `json:"awaits_reply"`
}

// Summary is a target's open comments in numbers.
type Summary struct {
	Open        int
	AwaitsReply bool
}

// Filter selects stored rows (comments and resolutions).
type Filter struct {
	Kind     string
	TargetID string
	AfterID  int64
	SinceAt  time.Time
}

// ListRequest reads one target's thread.
type ListRequest struct {
	Kind     string
	TargetID string
	OpenOnly bool
}

// AddRequest adds a comment; Section is a document heading's anchor.
type AddRequest struct {
	Kind     string
	TargetID string
	Section  string
	Body     string
	Actor    string
}

// ResolveRequest resolves a comment, with an optional note.
type ResolveRequest struct {
	ID    int64
	Note  string
	Actor string
}

// The domain's errors.
var (
	ErrNotFound = errors.New("comment not found")
	ErrInvalid  = errors.New("invalid comment")
)
