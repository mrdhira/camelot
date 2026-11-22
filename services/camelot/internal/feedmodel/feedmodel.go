// Package feedmodel holds the "what's new" feed: every card and document
// created or changed since Dhira last marked the feed seen.
package feedmodel

import (
	"errors"
	"fmt"
	"time"

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
	LatestAt   time.Time
}

// Changes counts the item's changes.
func (i *Item) Changes() int { return len(i.CardEvents) + len(i.DocEvents) }

// Feed is the changes after the seen marker, newest item first; Stamp is
// the newest event IDs it was read at, what "Mark all seen" records.
type Feed struct {
	Since     time.Time
	FirstLook bool
	AsOf      time.Time
	Stamp     Stamp
	Items     []*Item
}

// Stamp is the highest card and document event IDs at a moment: it changes
// with every change, and marks the feed seen without depending on clocks.
type Stamp struct {
	CardEventID int64
	DocEventID  int64
}

// String is the stamp as the pages carry it: c<card>-d<doc>.
func (s Stamp) String() string {
	return fmt.Sprintf("c%d-d%d", s.CardEventID, s.DocEventID)
}

// ParseStamp reads a stamp written by String.
func ParseStamp(raw string) (Stamp, error) {
	var s Stamp
	if _, err := fmt.Sscanf(raw, "c%d-d%d", &s.CardEventID, &s.DocEventID); err != nil || s.CardEventID < 0 || s.DocEventID < 0 {
		return Stamp{}, fmt.Errorf("%w: stamp %q", ErrInvalid, raw)
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
