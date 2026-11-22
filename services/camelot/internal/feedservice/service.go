// Package feedservice builds the "what's new" feed from the card and
// document histories and keeps its seen marker.
package feedservice

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/mrdhira/camelot/services/camelot/internal/commentmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/commentservice"
	"github.com/mrdhira/camelot/services/camelot/internal/docmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/docservice"
	"github.com/mrdhira/camelot/services/camelot/internal/feedmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/feedrepository"
	"github.com/mrdhira/camelot/services/camelot/internal/taskmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/taskservice"
)

// Service is the feed service.
type Service interface {
	// Feed syncs the documents and returns every change since the seen
	// marker (the last FirstLook before anything was marked seen).
	Feed(ctx context.Context) (*feedmodel.Feed, error)
	// MarkSeen moves the seen marker to the request's stamp (never back).
	MarkSeen(ctx context.Context, request *feedmodel.SeenRequest) error
	// Stamp syncs the documents and returns the newest card event, document event and comment IDs.
	Stamp(ctx context.Context) (feedmodel.Stamp, error)
}

type service struct {
	feedRepo   feedrepository.Repository
	taskSvc    taskservice.Service
	docSvc     docservice.Service
	commentSvc commentservice.Service
}

// New returns the Service.
func New(feedRepo feedrepository.Repository, taskSvc taskservice.Service, docSvc docservice.Service, commentSvc commentservice.Service) Service {
	return &service{feedRepo: feedRepo, taskSvc: taskSvc, docSvc: docSvc, commentSvc: commentSvc}
}

// Feed groups the card and document events since the marker per item.
func (s *service) Feed(ctx context.Context) (*feedmodel.Feed, error) {
	if err := s.docSvc.Sync(ctx); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	stamp, err := s.stamp(ctx)
	if err != nil {
		return nil, err
	}
	seen, err := s.feedRepo.SeenSelect(ctx)
	if err != nil {
		return nil, err
	}
	feed := &feedmodel.Feed{AsOf: now, Stamp: stamp}
	cardFilter := &taskmodel.EventFilter{}
	docFilter := &docmodel.EventFilter{}
	commentFilter := &commentmodel.Filter{}
	if seen == nil {
		feed.Since, feed.FirstLook = now.Add(-feedmodel.FirstLook), true
		cardFilter.Since, docFilter.Since, commentFilter.SinceAt = feed.Since, feed.Since, feed.Since
	} else {
		feed.Since = seen.At
		cardFilter.AfterID, docFilter.AfterID, commentFilter.AfterID = seen.Stamp.CardEventID, seen.Stamp.DocEventID, seen.Stamp.CommentID
	}
	comments, err := s.commentSvc.Changes(ctx, commentFilter)
	if err != nil {
		return nil, err
	}
	cardEvents, err := s.taskSvc.Events(ctx, cardFilter)
	if err != nil {
		return nil, err
	}
	docEvents, err := s.docSvc.Events(ctx, docFilter)
	if err != nil {
		return nil, err
	}
	cards, err := s.taskSvc.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	docs, err := s.docSvc.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	items := map[string]*feedmodel.Item{}
	item := func(kind, id string) *feedmodel.Item {
		key := kind + "/" + id
		if it, ok := items[key]; ok {
			return it
		}
		it := &feedmodel.Item{Kind: kind, ID: id, Title: id, Gone: true}
		items[key] = it
		return it
	}
	for _, e := range cardEvents {
		if e.ID > stamp.CardEventID {
			continue
		}
		it := item(feedmodel.KindCard, e.CardID)
		it.CardEvents = append(it.CardEvents, e)
		it.LatestAt = maxTime(it.LatestAt, e.CreatedAt)
	}
	for _, e := range docEvents {
		if e.ID > stamp.DocEventID {
			continue
		}
		it := item(feedmodel.KindDoc, e.DocID)
		it.DocEvents = append(it.DocEvents, e)
		it.LatestAt = maxTime(it.LatestAt, e.CreatedAt)
	}
	for _, c := range comments {
		if c.ID > stamp.CommentID {
			continue
		}
		kind := feedmodel.KindCard
		if c.Kind == commentmodel.KindDoc {
			kind = feedmodel.KindDoc
		}
		it := item(kind, c.TargetID)
		it.Comments = append([]*commentmodel.Comment{c}, it.Comments...)
		it.LatestAt = maxTime(it.LatestAt, c.CreatedAt)
	}
	for _, c := range cards {
		if it, ok := items[feedmodel.KindCard+"/"+c.ID]; ok {
			it.Title, it.Status, it.Gone = c.Title, c.Status.Label(), false
		}
	}
	for _, d := range docs.Docs {
		if it, ok := items[feedmodel.KindDoc+"/"+d.ID]; ok {
			it.Title, it.Status, it.Gone = d.Title, d.Status.Label(), false
		}
	}
	for _, it := range items {
		feed.Items = append(feed.Items, it)
	}
	slices.SortFunc(feed.Items, func(a, b *feedmodel.Item) int {
		if c := b.LatestAt.Compare(a.LatestAt); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	return feed, nil
}

// MarkSeen records the stamp the page showed.
func (s *service) MarkSeen(ctx context.Context, request *feedmodel.SeenRequest) error {
	stamp, err := feedmodel.ParseStamp(request.Stamp)
	if err != nil {
		return err
	}
	return s.feedRepo.SeenUpdate(ctx, &feedmodel.Seen{Stamp: stamp, At: time.Now().UTC()})
}

// Stamp syncs the documents and returns the newest event IDs.
func (s *service) Stamp(ctx context.Context) (feedmodel.Stamp, error) {
	if err := s.docSvc.Sync(ctx); err != nil {
		return feedmodel.Stamp{}, err
	}
	return s.stamp(ctx)
}

// stamp reads the newest card and document event IDs.
func (s *service) stamp(ctx context.Context) (feedmodel.Stamp, error) {
	cardID, err := s.taskSvc.EventMaxID(ctx)
	if err != nil {
		return feedmodel.Stamp{}, err
	}
	docID, err := s.docSvc.EventMaxID(ctx)
	if err != nil {
		return feedmodel.Stamp{}, err
	}
	commentID, err := s.commentSvc.MaxID(ctx)
	if err != nil {
		return feedmodel.Stamp{}, err
	}
	return feedmodel.Stamp{CardEventID: cardID, DocEventID: docID, CommentID: commentID}, nil
}

func maxTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}
