// Package feedmodel holds the "what's new" feed: every card and document
// created or changed since Dhira last marked the feed seen.
package feedmodel

import (
	"errors"
	"fmt"
	"time"

	"github.com/mrdhira/camelot/services/camelot/internal/commentmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/docmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/taskmodel"
)

// The kinds of feed item.
const (
	KindCard = "card"
	KindDoc  = "doc"
)

// FirstLook is how far back the feed reaches before anything was marked seen.
const FirstLook = 7 * 24 * time.Hour

// Item is one card or document with its changes in the window, newest first.
type Item struct {
	Kind       string
	ID         string
	Title      string
	Status     string
	Gone       bool
	CardEvents []*taskmodel.Event
	DocEvents  []*docmodel.Event
	Comments   []*commentmodel.Comment
	LatestAt   time.Time
}

// Changes counts the item's changes.
func (i *Item) Changes() int { return len(i.CardEvents) + len(i.DocEvents) + len(i.Comments) }

// Feed is the changes after the seen marker, newest item first; Stamp is
// the newest event IDs it was read at, what "Mark all seen" records.
type Feed struct {
	Since     time.Time
	FirstLook bool
	AsOf      time.Time
	Stamp     Stamp
	Items     []*Item
}

// Stamp is the highest card event, document event and comment IDs at a
// moment: it changes with every change, and marks the feed seen without
// depending on clocks.
type Stamp struct {
	CardEventID int64
	DocEventID  int64
	CommentID   int64
}

// String is the stamp as the pages carry it: c<card>-d<doc>-m<comment>.
func (s Stamp) String() string {
	return fmt.Sprintf("c%d-d%d-m%d", s.CardEventID, s.DocEventID, s.CommentID)
}

// ParseStamp reads a stamp written by String.
func ParseStamp(raw string) (Stamp, error) {
	var s Stamp
	n, err := fmt.Sscanf(raw, "c%d-d%d-m%d", &s.CardEventID, &s.DocEventID, &s.CommentID)
	if err != nil || n != 3 || s.CardEventID < 0 || s.DocEventID < 0 || s.CommentID < 0 || s.String() != raw {
		return Stamp{}, fmt.Errorf("%w: stamp %q (like c12-d3-m4)", ErrInvalid, raw)
	}
	return s, nil
}

// Seen is the feed's marker: the stamp last marked seen and when.
type Seen struct {
	Stamp Stamp
	At    time.Time
}

// SeenRequest marks the feed seen up to a stamp the page showed.
type SeenRequest struct {
	Stamp string
}

// ErrInvalid is a malformed feed request.
var ErrInvalid = errors.New("invalid feed request")
