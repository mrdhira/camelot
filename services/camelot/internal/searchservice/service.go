// Package searchservice finds cards (ID, title) and documents (ID, title, text).
package searchservice

import (
	"context"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mrdhira/camelot/services/camelot/internal/docservice"
	"github.com/mrdhira/camelot/services/camelot/internal/searchmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/taskmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/taskservice"
)

// recentCards is how many cards an empty query lists.
const recentCards = 8

// snippetContext is how many bytes of text a snippet shows on each side of the match.
const snippetContext = 40

// Service is the search service.
type Service interface {
	// Search returns cards, then documents, matching the request.
	Search(ctx context.Context, request *searchmodel.Request) ([]searchmodel.Result, error)
}

type service struct {
	taskSvc taskservice.Service
	docSvc  docservice.Service
}

// New returns the Service.
func New(taskSvc taskservice.Service, docSvc docservice.Service) Service {
	return &service{taskSvc: taskSvc, docSvc: docSvc}
}

// Search matches cards and documents.
func (s *service) Search(ctx context.Context, request *searchmodel.Request) ([]searchmodel.Result, error) {
	q := strings.TrimSpace(request.Query)
	cards, err := s.taskSvc.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	out := []searchmodel.Result{}
	if q == "" {
		slices.SortFunc(cards, func(a, b *taskmodel.Card) int { return b.UpdatedAt.Compare(a.UpdatedAt) })
		for _, c := range cards[:min(len(cards), recentCards)] {
			out = append(out, cardResult(c))
		}
		return out, nil
	}
	for _, c := range cards {
		if contains(c.ID, q) || contains(c.Title, q) {
			out = append(out, cardResult(c))
		}
	}
	docs, err := s.docSvc.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	for _, d := range docs.Docs {
		r := searchmodel.Result{Kind: searchmodel.KindDoc, ID: d.ID, Title: d.Title, Detail: d.Type.Short() + " · " + d.Status.Label()}
		if contains(d.ID, q) || contains(d.Title, q) {
			out = append(out, r)
		} else if snippet, ok := Snippet(string(d.Body), q); ok {
			r.Detail = snippet
			out = append(out, r)
		}
	}
	return out[:min(len(out), searchmodel.MaxResults)], nil
}

// cardResult is a card as a result.
func cardResult(c *taskmodel.Card) searchmodel.Result {
	return searchmodel.Result{Kind: searchmodel.KindCard, ID: c.ID, Title: c.Title, Detail: c.Status.Label() + " · " + string(c.Type)}
}

// contains reports whether text contains q, case-insensitively.
func contains(text, q string) bool {
	_, _, ok := find(text, q)
	return ok
}

// Snippet returns the text around the first case-insensitive match of q,
// cut on rune boundaries of the original text, whitespace collapsed.
func Snippet(text, q string) (string, bool) {
	start, end, ok := find(text, q)
	if !ok {
		return "", false
	}
	from, to := max(0, start-snippetContext), min(len(text), end+snippetContext)
	for from > 0 && !utf8.RuneStart(text[from]) {
		from--
	}
	for to < len(text) && !utf8.RuneStart(text[to]) {
		to++
	}
	return "…" + strings.Join(strings.Fields(text[from:to]), " ") + "…", true
}

// find returns the byte range in text of the first case-insensitive match
// of q. Matching runs on a rune-by-rune lower-cased copy whose every byte
// maps back to the original rune it came from, so the range is always
// valid in text, whatever the case mapping does to byte lengths.
func find(text, q string) (int, int, bool) {
	lq := strings.Map(unicode.ToLower, q)
	if lq == "" {
		return 0, 0, false
	}
	var lower strings.Builder
	origin := make([]int, 0, len(text))
	for i, r := range text {
		l := string(unicode.ToLower(r))
		lower.WriteString(l)
		for range len(l) {
			origin = append(origin, i)
		}
	}
	at := strings.Index(lower.String(), lq)
	if at < 0 {
		return 0, 0, false
	}
	last := origin[at+len(lq)-1]
	_, size := utf8.DecodeRuneInString(text[last:])
	return origin[at], last + size, true
}
