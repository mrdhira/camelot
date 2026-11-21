// Package docservice lists documents, builds their chains and changes
// their status.
package docservice

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/mrdhira/camelot/services/camelot/internal/docmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/docrepository"
)

// Service is the document service.
type Service interface {
	// List returns the documents matching the request, chain order
	// (type, then ID), and every problem found in the files and links.
	List(ctx context.Context, request *docmodel.ListRequest) (*docmodel.ListResponse, error)
	// Get returns one document with its chain; docmodel.ErrNotFound when absent.
	Get(ctx context.Context, request *docmodel.GetRequest) (*docmodel.Chain, error)
	// SetStatus changes a document's status and sets its updated date to today in the configured zone.
	SetStatus(ctx context.Context, request *docmodel.SetStatusRequest) (*docmodel.Doc, error)
	// Exists reports which of ids name a document.
	Exists(ctx context.Context, ids []string) (map[string]bool, error)
}

// Config is the service's settings.
type Config struct {
	// Zone is where "today" is for a document's updated date; nil means UTC.
	Zone *time.Location
}

type service struct {
	config  Config
	docRepo docrepository.Repository
}

// New returns the Service on docRepo.
func New(config Config, docRepo docrepository.Repository) Service {
	if config.Zone == nil {
		config.Zone = time.UTC
	}
	return &service{config: config, docRepo: docRepo}
}

// List reads every document and checks the links between them.
func (s *service) List(ctx context.Context, request *docmodel.ListRequest) (*docmodel.ListResponse, error) {
	all, problems, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	docs := []*docmodel.Doc{}
	for _, d := range all {
		if request != nil && len(request.Types) > 0 && !slices.Contains(request.Types, d.Type) {
			continue
		}
		if request != nil && len(request.Statuses) > 0 && !slices.Contains(request.Statuses, d.Status) {
			continue
		}
		docs = append(docs, d)
	}
	return &docmodel.ListResponse{Docs: docs, Problems: problems}, nil
}

// Get returns the document and walks its chain both ways.
func (s *service) Get(ctx context.Context, request *docmodel.GetRequest) (*docmodel.Chain, error) {
	id, err := docmodel.ParseID(request.ID)
	if err != nil {
		return nil, err
	}
	all, _, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	byID := index(all)
	doc, ok := byID[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", docmodel.ErrNotFound, id)
	}
	chain := &docmodel.Chain{Doc: doc}
	seen := map[string]bool{doc.ID: true}
	queue := slices.Clone(doc.Serves)
	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		if seen[next] {
			continue
		}
		seen[next] = true
		if up, ok := byID[next]; ok {
			chain.Up = append(chain.Up, up)
			queue = append(queue, up.Serves...)
		}
	}
	seen = map[string]bool{doc.ID: true}
	queue = []string{doc.ID}
	for len(queue) > 0 {
		parent := queue[0]
		queue = queue[1:]
		for _, d := range all {
			if !seen[d.ID] && slices.Contains(d.Serves, parent) {
				seen[d.ID] = true
				chain.Down = append(chain.Down, d)
				queue = append(queue, d.ID)
			}
		}
	}
	sortChain(chain.Up)
	sortChain(chain.Down)
	return chain, nil
}

// SetStatus rewrites the status and updated keys of the document's file.
func (s *service) SetStatus(ctx context.Context, request *docmodel.SetStatusRequest) (*docmodel.Doc, error) {
	status, err := docmodel.ParseStatus(request.Status)
	if err != nil {
		return nil, err
	}
	chain, err := s.Get(ctx, &docmodel.GetRequest{ID: request.ID})
	if err != nil {
		return nil, err
	}
	doc := chain.Doc
	today := time.Now().In(s.config.Zone).Format(docmodel.DateLayout)
	fields := map[string]string{docmodel.KeyStatus: string(status), docmodel.KeyUpdated: today}
	if err := s.docRepo.DocUpdateFields(ctx, doc.Path, fields); err != nil {
		return nil, err
	}
	doc.Status = status
	doc.Updated = today
	return doc, nil
}

// Exists reports which ids name a document.
func (s *service) Exists(ctx context.Context, ids []string) (map[string]bool, error) {
	all, _, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	byID := index(all)
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		_, out[id] = byID[id]
	}
	return out, nil
}

// load reads every document, drops duplicates of an ID (keeping the
// first by path) and reports duplicate IDs and broken or upside-down
// serves links as problems.
func (s *service) load(ctx context.Context) ([]*docmodel.Doc, []docmodel.Problem, error) {
	docs, problems, err := s.docRepo.DocSelect(ctx)
	if err != nil {
		return nil, nil, err
	}
	slices.SortFunc(docs, func(a, b *docmodel.Doc) int { return strings.Compare(a.Path, b.Path) })
	byID := map[string]*docmodel.Doc{}
	kept := docs[:0]
	for _, d := range docs {
		if first, ok := byID[d.ID]; ok {
			problems = append(problems, docmodel.Problem{Path: d.Path, DocID: d.ID, Message: "duplicate document ID, also in " + first.Path + "; ignored"})
			continue
		}
		byID[d.ID] = d
		kept = append(kept, d)
	}
	for _, d := range kept {
		for _, target := range d.Serves {
			up, ok := byID[target]
			switch {
			case !ok:
				problems = append(problems, docmodel.Problem{Path: d.Path, DocID: d.ID, Message: "serves " + target + ", which is not a document"})
			case target == d.ID:
				problems = append(problems, docmodel.Problem{Path: d.Path, DocID: d.ID, Message: "serves itself"})
			case up.Type.Rank() >= d.Type.Rank():
				problems = append(problems, docmodel.Problem{Path: d.Path, DocID: d.ID, Message: fmt.Sprintf("a %s serves a %s: the chain runs TRD → PRD → BRD → OKR, KPI → OKR", d.Type.Short(), up.Type.Short())})
			}
		}
	}
	sortChain(kept)
	return kept, problems, nil
}

// index maps documents by ID.
func index(docs []*docmodel.Doc) map[string]*docmodel.Doc {
	byID := make(map[string]*docmodel.Doc, len(docs))
	for _, d := range docs {
		byID[d.ID] = d
	}
	return byID
}

// sortChain orders documents by type rank, then ID.
func sortChain(docs []*docmodel.Doc) {
	slices.SortFunc(docs, func(a, b *docmodel.Doc) int {
		if a.Type.Rank() != b.Type.Rank() {
			return a.Type.Rank() - b.Type.Rank()
		}
		return strings.Compare(a.ID, b.ID)
	})
}
