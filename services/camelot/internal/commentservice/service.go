// Package commentservice keeps the comments on cards and documents.
package commentservice

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mrdhira/camelot/services/camelot/internal/commentmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/commentrepository"
	"github.com/mrdhira/camelot/services/camelot/internal/docmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/docservice"
	"github.com/mrdhira/camelot/services/camelot/internal/pkg/markdown"
	"github.com/mrdhira/camelot/services/camelot/internal/taskmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/taskservice"
)

// Service is the comment service.
type Service interface {
	// Add comments on an existing card or document (a document section must be one of its headings).
	Add(ctx context.Context, request *commentmodel.AddRequest) (*commentmodel.Comment, error)
	// Resolve closes an open comment; resolving it again changes nothing.
	Resolve(ctx context.Context, request *commentmodel.ResolveRequest) (*commentmodel.Comment, error)
	// Thread returns an existing target's comments, oldest first.
	Thread(ctx context.Context, request *commentmodel.ListRequest) (*commentmodel.Thread, error)
	// Summaries returns the open comments per target of one kind (targets with none are absent).
	Summaries(ctx context.Context, kind string) (map[string]commentmodel.Summary, error)
	// Changes returns the stored rows (comments and resolutions) matching the filter, oldest first.
	Changes(ctx context.Context, filter *commentmodel.Filter) ([]*commentmodel.Comment, error)
	// MaxID returns the highest comment row ID; 0 when none.
	MaxID(ctx context.Context) (int64, error)
}

type service struct {
	commentRepo commentrepository.Repository
	taskSvc     taskservice.Service
	docSvc      docservice.Service
}

// New returns the Service.
func New(commentRepo commentrepository.Repository, taskSvc taskservice.Service, docSvc docservice.Service) Service {
	return &service{commentRepo: commentRepo, taskSvc: taskSvc, docSvc: docSvc}
}

// Add stores a comment after checking its target, section, text and actor.
func (s *service) Add(ctx context.Context, request *commentmodel.AddRequest) (*commentmodel.Comment, error) {
	targetID, err := s.target(ctx, request.Kind, request.TargetID, request.Section)
	if err != nil {
		return nil, err
	}
	body, err := parseBody(request.Body, true)
	if err != nil {
		return nil, err
	}
	actor, err := parseActor(request.Actor)
	if err != nil {
		return nil, err
	}
	c := &commentmodel.Comment{Kind: request.Kind, TargetID: targetID, Section: strings.TrimSpace(request.Section), Body: body, Actor: actor, CreatedAt: time.Now().UTC()}
	if err := s.commentRepo.CommentInsert(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// Resolve stores a resolution row for an open comment.
func (s *service) Resolve(ctx context.Context, request *commentmodel.ResolveRequest) (*commentmodel.Comment, error) {
	c, err := s.commentRepo.CommentGet(ctx, request.ID)
	if err != nil {
		return nil, err
	}
	if c.ResolvesID != 0 {
		return nil, fmt.Errorf("%w: %d is a resolution, not a comment", commentmodel.ErrInvalid, c.ID)
	}
	note, err := parseBody(request.Note, false)
	if err != nil {
		return nil, err
	}
	actor, err := parseActor(request.Actor)
	if err != nil {
		return nil, err
	}
	resolution := &commentmodel.Comment{Kind: c.Kind, TargetID: c.TargetID, Section: c.Section, Body: note, Actor: actor, CreatedAt: time.Now().UTC(), ResolvesID: c.ID}
	if err := s.commentRepo.CommentInsert(ctx, resolution); err != nil {
		return nil, err
	}
	thread, err := s.Thread(ctx, &commentmodel.ListRequest{Kind: c.Kind, TargetID: c.TargetID})
	if err != nil {
		return nil, err
	}
	for _, t := range thread.Comments {
		if t.ID == c.ID {
			return t, nil
		}
	}
	return nil, fmt.Errorf("%w: %d", commentmodel.ErrNotFound, c.ID)
}

// Thread folds a target's rows into its comments.
func (s *service) Thread(ctx context.Context, request *commentmodel.ListRequest) (*commentmodel.Thread, error) {
	targetID, err := s.target(ctx, request.Kind, request.TargetID, "")
	if err != nil {
		return nil, err
	}
	rows, err := s.commentRepo.CommentSelect(ctx, &commentmodel.Filter{Kind: request.Kind, TargetID: targetID})
	if err != nil {
		return nil, err
	}
	thread := fold(rows)[targetID]
	if thread == nil {
		thread = &commentmodel.Thread{Comments: []*commentmodel.Comment{}}
	}
	if request.OpenOnly {
		open := []*commentmodel.Comment{}
		for _, c := range thread.Comments {
			if c.Open() {
				open = append(open, c)
			}
		}
		thread.Comments = open
	}
	return thread, nil
}

// Summaries folds every row of one kind.
func (s *service) Summaries(ctx context.Context, kind string) (map[string]commentmodel.Summary, error) {
	rows, err := s.commentRepo.CommentSelect(ctx, &commentmodel.Filter{Kind: kind})
	if err != nil {
		return nil, err
	}
	out := map[string]commentmodel.Summary{}
	for id, t := range fold(rows) {
		if t.Open > 0 {
			out[id] = commentmodel.Summary{Open: t.Open, AwaitsReply: t.AwaitsReply}
		}
	}
	return out, nil
}

// Changes returns the stored rows.
func (s *service) Changes(ctx context.Context, filter *commentmodel.Filter) ([]*commentmodel.Comment, error) {
	return s.commentRepo.CommentSelect(ctx, filter)
}

// MaxID returns the highest row ID.
func (s *service) MaxID(ctx context.Context) (int64, error) {
	return s.commentRepo.CommentMaxID(ctx)
}

// fold groups rows (one kind) per target: comments with their resolutions applied.
func fold(rows []*commentmodel.Comment) map[string]*commentmodel.Thread {
	byID := map[int64]*commentmodel.Comment{}
	threads := map[string]*commentmodel.Thread{}
	for _, r := range rows {
		if r.ResolvesID == 0 {
			byID[r.ID] = r
			t := threads[r.TargetID]
			if t == nil {
				t = &commentmodel.Thread{}
				threads[r.TargetID] = t
			}
			t.Comments = append(t.Comments, r)
			continue
		}
		if c, ok := byID[r.ResolvesID]; ok {
			at := r.CreatedAt
			c.ResolvedBy, c.ResolvedAt, c.ResolvedNote = r.Actor, &at, r.Body
		}
	}
	for _, t := range threads {
		for _, c := range t.Comments {
			if c.Open() {
				t.Open++
			}
		}
		newest := t.Comments[len(t.Comments)-1]
		t.AwaitsReply = newest.Open() && newest.Actor == commentmodel.Owner
	}
	return threads
}

// target checks the card or document (and section) exists and returns its canonical ID.
func (s *service) target(ctx context.Context, kind, id, section string) (string, error) {
	switch kind {
	case commentmodel.KindCard:
		if strings.TrimSpace(section) != "" {
			return "", fmt.Errorf("%w: sections are for documents only", commentmodel.ErrInvalid)
		}
		detail, err := s.taskSvc.Get(ctx, &taskmodel.GetRequest{ID: id})
		if err != nil {
			return "", err
		}
		return detail.Card.ID, nil
	case commentmodel.KindDoc:
		chain, err := s.docSvc.Get(ctx, &docmodel.GetRequest{ID: id})
		if err != nil {
			return "", err
		}
		if section = strings.TrimSpace(section); section != "" {
			html, err := markdown.Render(chain.Doc.Body)
			if err != nil {
				return "", err
			}
			var ids []string
			for _, h := range markdown.Headings(html) {
				if h.ID == section {
					return chain.Doc.ID, nil
				}
				ids = append(ids, h.ID)
			}
			return "", fmt.Errorf("%w: section %q is not a heading of %s (headings: %s)", commentmodel.ErrInvalid, section, chain.Doc.ID, strings.Join(ids, ", "))
		}
		return chain.Doc.ID, nil
	}
	return "", fmt.Errorf("%w: kind %q (card or doc)", commentmodel.ErrInvalid, kind)
}

// parseBody trims a comment's text and checks its length.
func parseBody(raw string, required bool) (string, error) {
	body := strings.TrimSpace(strings.ReplaceAll(raw, "\r\n", "\n"))
	if required && body == "" {
		return "", fmt.Errorf("%w: the comment is empty", commentmodel.ErrInvalid)
	}
	if n := utf8.RuneCountInString(body); n > commentmodel.MaxBody {
		return "", fmt.Errorf("%w: %d characters, at most %d", commentmodel.ErrInvalid, n, commentmodel.MaxBody)
	}
	if !utf8.ValidString(body) {
		return "", fmt.Errorf("%w: the text is not valid UTF-8", commentmodel.ErrInvalid)
	}
	return body, nil
}

// parseActor reads who comments.
func parseActor(raw string) (string, error) {
	actor, err := taskmodel.ParseOneLine("actor", raw)
	if err != nil {
		return "", err
	}
	if actor == "" {
		return "", fmt.Errorf("%w: actor is required", commentmodel.ErrInvalid)
	}
	return actor, nil
}
