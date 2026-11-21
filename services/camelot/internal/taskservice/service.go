// Package taskservice creates, changes, moves and deletes cards, recording
// every change as an event.
package taskservice

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/mrdhira/camelot/services/camelot/internal/docmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/docservice"
	"github.com/mrdhira/camelot/services/camelot/internal/taskmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/taskrepository"
)

// Service is the card service.
type Service interface {
	// Create stores a new card and its "created" event.
	Create(ctx context.Context, request *taskmodel.CreateRequest) (*taskmodel.Card, error)
	// Update changes the set fields, one event per changed field.
	Update(ctx context.Context, request *taskmodel.UpdateRequest) (*taskmodel.Card, error)
	// Delete removes a card nothing depends on and no card belongs to.
	Delete(ctx context.Context, request *taskmodel.DeleteRequest) error
	// Get returns a card with its links and history.
	Get(ctx context.Context, request *taskmodel.GetRequest) (*taskmodel.GetResponse, error)
	// List returns the cards matching the filter.
	List(ctx context.Context, request *taskmodel.ListRequest) ([]*taskmodel.Card, error)
	// Events returns events matching the filter, newest first.
	Events(ctx context.Context, filter *taskmodel.EventFilter) ([]*taskmodel.Event, error)
	// Files lists the Markdown files of a card's folder (none without a folder).
	Files(ctx context.Context, request *taskmodel.FilesRequest) ([]string, error)
	// File reads one Markdown file of a card's folder.
	File(ctx context.Context, request *taskmodel.FileRequest) (*taskmodel.FileResponse, error)
}

type service struct {
	taskRepo   taskrepository.Repository
	folderRepo taskrepository.Folders
	docSvc     docservice.Service
}

// New returns the Service.
func New(taskRepo taskrepository.Repository, folderRepo taskrepository.Folders, docSvc docservice.Service) Service {
	return &service{taskRepo: taskRepo, folderRepo: folderRepo, docSvc: docSvc}
}

// Files lists the card's folder files.
func (s *service) Files(ctx context.Context, request *taskmodel.FilesRequest) ([]string, error) {
	card, err := s.card(ctx, request.ID)
	if err != nil {
		return nil, err
	}
	if card.Folder == "" {
		return []string{}, nil
	}
	return s.folderRepo.FolderFiles(ctx, card.Folder)
}

// File reads one file of the card's folder.
func (s *service) File(ctx context.Context, request *taskmodel.FileRequest) (*taskmodel.FileResponse, error) {
	card, err := s.card(ctx, request.ID)
	if err != nil {
		return nil, err
	}
	if card.Folder == "" {
		return nil, fmt.Errorf("%w: %s has no folder", taskmodel.ErrFileNotFound, card.ID)
	}
	content, err := s.folderRepo.FolderFileRead(ctx, card.Folder, request.Name)
	if err != nil {
		return nil, err
	}
	return &taskmodel.FileResponse{Card: card, Name: request.Name, Content: content}, nil
}

// card returns the live card id or taskmodel.ErrNotFound.
func (s *service) card(ctx context.Context, rawID string) (*taskmodel.Card, error) {
	id, err := taskmodel.ParseCardID(rawID)
	if err != nil {
		return nil, err
	}
	card, err := s.taskRepo.CardSelectByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if card == nil {
		return nil, fmt.Errorf("%w: %s", taskmodel.ErrNotFound, id)
	}
	return card, nil
}

// Create validates the request, takes the next number for a bare prefix,
// and stores the card with its "created" event (or "imported", whose time
// is the request's CreatedAt).
func (s *service) Create(ctx context.Context, request *taskmodel.CreateRequest) (*taskmodel.Card, error) {
	actor, err := parseActor(request.Actor)
	if err != nil {
		return nil, err
	}
	id, err := s.newID(ctx, request.ID, request.Prefix)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	card := &taskmodel.Card{ID: id, Type: taskmodel.TypeTask, Status: taskmodel.StatusBacklog, Deps: []string{}, Docs: []string{}, CreatedAt: now, UpdatedAt: now}
	if !request.CreatedAt.IsZero() {
		card.CreatedAt = request.CreatedAt.UTC()
	}
	if card.Title, err = taskmodel.ParseTitle(request.Title); err != nil {
		return nil, err
	}
	if request.Type != "" {
		if card.Type, err = taskmodel.ParseType(request.Type); err != nil {
			return nil, err
		}
	}
	if request.Status != "" {
		if card.Status, err = taskmodel.ParseStatus(request.Status); err != nil {
			return nil, err
		}
	}
	if card.Assignee, err = taskmodel.ParseOneLine("assignee", request.Assignee); err != nil {
		return nil, err
	}
	if card.Waiting, err = taskmodel.ParseOneLine("waiting", request.Waiting); err != nil {
		return nil, err
	}
	if card.Folder, err = parseFolder(request.Folder); err != nil {
		return nil, err
	}
	if card.Description, err = taskmodel.ParseDescription(request.Description); err != nil {
		return nil, err
	}
	all, err := s.taskRepo.CardSelect(ctx, nil)
	if err != nil {
		return nil, err
	}
	if card.MilestoneID, err = parseMilestone(request.MilestoneID, card.ID, all); err != nil {
		return nil, err
	}
	if card.Deps, err = parseDeps(request.Deps, card.ID, all, nil); err != nil {
		return nil, err
	}
	if card.Docs, err = s.parseDocs(ctx, request.Docs, nil); err != nil {
		return nil, err
	}
	field, at := taskmodel.FieldCreated, now
	if request.Imported {
		field, at = taskmodel.FieldImported, card.CreatedAt
	}
	event := taskmodel.NewEvent(card.ID, actor, field, "", string(card.Status), request.Note, at)
	if err := s.taskRepo.CardInsert(ctx, card, []*taskmodel.Event{event}); err != nil {
		return nil, err
	}
	return card, nil
}

// Update applies every set field, refuses a stale read, and records one
// event per field that changed.
func (s *service) Update(ctx context.Context, request *taskmodel.UpdateRequest) (*taskmodel.Card, error) {
	actor, err := parseActor(request.Actor)
	if err != nil {
		return nil, err
	}
	id, err := taskmodel.ParseCardID(request.ID)
	if err != nil {
		return nil, err
	}
	current, err := s.taskRepo.CardSelectByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("%w: %s", taskmodel.ErrNotFound, id)
	}
	if !request.UpdatedAt.IsZero() && !request.UpdatedAt.Equal(current.UpdatedAt) {
		return nil, fmt.Errorf("%w (%s)", taskmodel.ErrStale, id)
	}
	next := *current
	now := time.Now().UTC()
	var events []*taskmodel.Event
	record := func(field, from, to string) {
		if from != to {
			events = append(events, taskmodel.NewEvent(id, actor, field, from, to, request.Note, now))
		}
	}
	if request.Title != nil {
		if next.Title, err = taskmodel.ParseTitle(*request.Title); err != nil {
			return nil, err
		}
		record(taskmodel.FieldTitle, current.Title, next.Title)
	}
	if request.Type != nil {
		if next.Type, err = taskmodel.ParseType(*request.Type); err != nil {
			return nil, err
		}
		if current.Type == taskmodel.TypeMilestone && next.Type != taskmodel.TypeMilestone {
			children, err := s.taskRepo.CardSelect(ctx, &taskmodel.CardFilter{MilestoneID: id})
			if err != nil {
				return nil, err
			}
			if len(children) > 0 {
				return nil, fmt.Errorf("%w: %s still has %d cards; move them out before it stops being a milestone", taskmodel.ErrInvalidLink, id, len(children))
			}
		}
		record(taskmodel.FieldType, string(current.Type), string(next.Type))
	}
	if request.Status != nil {
		if next.Status, err = taskmodel.ParseStatus(*request.Status); err != nil {
			return nil, err
		}
		record(taskmodel.FieldStatus, string(current.Status), string(next.Status))
	}
	if request.Assignee != nil {
		if next.Assignee, err = taskmodel.ParseOneLine("assignee", *request.Assignee); err != nil {
			return nil, err
		}
		record(taskmodel.FieldAssignee, current.Assignee, next.Assignee)
	}
	if request.Waiting != nil {
		if next.Waiting, err = taskmodel.ParseOneLine("waiting", *request.Waiting); err != nil {
			return nil, err
		}
		record(taskmodel.FieldWaiting, current.Waiting, next.Waiting)
	}
	if request.Folder != nil {
		if next.Folder, err = parseFolder(*request.Folder); err != nil {
			return nil, err
		}
		record(taskmodel.FieldFolder, current.Folder, next.Folder)
	}
	if request.Description != nil {
		if next.Description, err = taskmodel.ParseDescription(*request.Description); err != nil {
			return nil, err
		}
		if next.Description != current.Description {
			events = append(events, taskmodel.NewEvent(id, actor, taskmodel.FieldDescription, "", "", request.Note, now))
		}
	}
	if request.MilestoneID != nil || request.Deps != nil {
		all, err := s.taskRepo.CardSelect(ctx, nil)
		if err != nil {
			return nil, err
		}
		if request.MilestoneID != nil && strings.TrimSpace(*request.MilestoneID) != current.MilestoneID {
			if next.MilestoneID, err = parseMilestone(*request.MilestoneID, id, all); err != nil {
				return nil, err
			}
			record(taskmodel.FieldMilestone, current.MilestoneID, next.MilestoneID)
		}
		if request.Deps != nil {
			if next.Deps, err = parseDeps(*request.Deps, id, all, current.Deps); err != nil {
				return nil, err
			}
			record(taskmodel.FieldDeps, strings.Join(current.Deps, ", "), strings.Join(next.Deps, ", "))
		}
	}
	if request.Docs != nil {
		if next.Docs, err = s.parseDocs(ctx, *request.Docs, current.Docs); err != nil {
			return nil, err
		}
		record(taskmodel.FieldDocs, strings.Join(current.Docs, ", "), strings.Join(next.Docs, ", "))
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("%w (%s)", taskmodel.ErrNothingToChange, id)
	}
	next.UpdatedAt = now
	updated, err := s.taskRepo.CardUpdate(ctx, &next, current.UpdatedAt, events)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, fmt.Errorf("%w (%s)", taskmodel.ErrStale, id)
	}
	return updated, nil
}

// Delete refuses a card others depend on or belong to.
func (s *service) Delete(ctx context.Context, request *taskmodel.DeleteRequest) error {
	actor, err := parseActor(request.Actor)
	if err != nil {
		return err
	}
	id, err := taskmodel.ParseCardID(request.ID)
	if err != nil {
		return err
	}
	all, err := s.taskRepo.CardSelect(ctx, nil)
	if err != nil {
		return err
	}
	var holders []string
	found := false
	for _, c := range all {
		if c.ID == id {
			found = true
		}
		if c.MilestoneID == id || slices.Contains(c.Deps, id) {
			holders = append(holders, c.ID)
		}
	}
	if !found {
		return fmt.Errorf("%w: %s", taskmodel.ErrNotFound, id)
	}
	if len(holders) > 0 {
		return fmt.Errorf("%w: %s is a dependency or milestone of %s; unlink them first", taskmodel.ErrInvalidLink, id, strings.Join(holders, ", "))
	}
	now := time.Now().UTC()
	deleted, err := s.taskRepo.CardDelete(ctx, id, now, taskmodel.NewEvent(id, actor, taskmodel.FieldDeleted, "", "", request.Note, now))
	if err != nil {
		return err
	}
	if !deleted {
		return fmt.Errorf("%w: %s", taskmodel.ErrNotFound, id)
	}
	return nil
}

// Get returns the card, its dependencies' states, the cards that depend on
// it or belong to it, and its history.
func (s *service) Get(ctx context.Context, request *taskmodel.GetRequest) (*taskmodel.GetResponse, error) {
	id, err := taskmodel.ParseCardID(request.ID)
	if err != nil {
		return nil, err
	}
	all, err := s.taskRepo.CardSelect(ctx, nil)
	if err != nil {
		return nil, err
	}
	byID := map[string]*taskmodel.Card{}
	for _, c := range all {
		byID[c.ID] = c
	}
	card, ok := byID[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", taskmodel.ErrNotFound, id)
	}
	response := &taskmodel.GetResponse{Card: card}
	for _, dep := range card.Deps {
		state := taskmodel.DepState{ID: dep}
		if d, ok := byID[dep]; ok {
			state.Title, state.Status, state.Found = d.Title, d.Status, true
		}
		response.Deps = append(response.Deps, state)
	}
	for _, c := range all {
		if slices.Contains(c.Deps, id) {
			response.Dependents = append(response.Dependents, c)
		}
		if c.MilestoneID == id {
			response.Children = append(response.Children, c)
		}
	}
	if response.Events, err = s.taskRepo.CardEventSelect(ctx, &taskmodel.EventFilter{CardID: id}); err != nil {
		return nil, err
	}
	return response, nil
}

// List returns the matching cards.
func (s *service) List(ctx context.Context, request *taskmodel.ListRequest) ([]*taskmodel.Card, error) {
	var filter *taskmodel.CardFilter
	if request != nil {
		filter = &request.Filter
	}
	return s.taskRepo.CardSelect(ctx, filter)
}

// Events returns the matching events.
func (s *service) Events(ctx context.Context, filter *taskmodel.EventFilter) ([]*taskmodel.Event, error) {
	return s.taskRepo.CardEventSelect(ctx, filter)
}

// newID returns the requested ID, or the next number after the highest
// "<prefix>-N" (at least three digits).
func (s *service) newID(ctx context.Context, rawID, rawPrefix string) (string, error) {
	if rawID != "" {
		return taskmodel.ParseCardID(rawID)
	}
	if rawPrefix == "" {
		return "", fmt.Errorf("%w: give an ID or a prefix", taskmodel.ErrInvalidID)
	}
	prefix, err := taskmodel.ParsePrefix(rawPrefix)
	if err != nil {
		return "", err
	}
	n, err := s.taskRepo.CardMaxNumber(ctx, prefix)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%03d", prefix, n+1), nil
}

// parseDocs reads document IDs and refuses any new one (not in kept) that
// names no document; a kept link to a renamed file stays, for check to report.
func (s *service) parseDocs(ctx context.Context, raw, kept []string) ([]string, error) {
	ids, err := taskmodel.ParseIDList(raw, docmodel.ParseID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", taskmodel.ErrInvalidLink, err)
	}
	if len(ids) == 0 {
		return ids, nil
	}
	exists, err := s.docSvc.Exists(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if !exists[id] && !slices.Contains(kept, id) {
			return nil, fmt.Errorf("%w: document %s does not exist", taskmodel.ErrInvalidLink, id)
		}
	}
	return ids, nil
}

// parseActor reads who makes a change.
func parseActor(raw string) (string, error) {
	actor, err := taskmodel.ParseOneLine("actor", raw)
	if err != nil {
		return "", err
	}
	if actor == "" {
		return "", fmt.Errorf("%w: actor is required", taskmodel.ErrInvalidField)
	}
	return actor, nil
}

// parseFolder reads a card folder: a clean path relative to the vault, under tasks/.
func parseFolder(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", nil
	}
	clean := path.Clean(strings.TrimSuffix(v, "/"))
	if clean != v && clean+"/" != v || !strings.HasPrefix(clean, "tasks/") || strings.Contains(clean, "..") {
		return "", fmt.Errorf("%w: folder %q must be a clean path under tasks/", taskmodel.ErrInvalidField, raw)
	}
	return clean, nil
}

// parseMilestone reads a milestone: empty, or a live milestone card other than the card itself.
func parseMilestone(raw, cardID string, all []*taskmodel.Card) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	id, err := taskmodel.ParseCardID(raw)
	if err != nil {
		return "", err
	}
	if id == cardID {
		return "", fmt.Errorf("%w: a card cannot be its own milestone", taskmodel.ErrInvalidLink)
	}
	for _, c := range all {
		if c.ID == id {
			if c.Type != taskmodel.TypeMilestone {
				return "", fmt.Errorf("%w: %s is a %s, not a milestone", taskmodel.ErrInvalidLink, id, c.Type)
			}
			return id, nil
		}
	}
	return "", fmt.Errorf("%w: milestone %s does not exist", taskmodel.ErrInvalidLink, id)
}

// parseDeps reads dependencies: live cards (a kept one is not re-checked),
// not the card itself, closing no cycle.
func parseDeps(raw []string, cardID string, all []*taskmodel.Card, kept []string) ([]string, error) {
	deps, err := taskmodel.ParseIDList(raw, taskmodel.ParseCardID)
	if err != nil {
		return nil, err
	}
	graph := map[string][]string{}
	for _, c := range all {
		graph[c.ID] = c.Deps
	}
	for _, dep := range deps {
		if dep == cardID {
			return nil, fmt.Errorf("%w: a card cannot depend on itself", taskmodel.ErrInvalidLink)
		}
		if _, ok := graph[dep]; !ok && !slices.Contains(kept, dep) {
			return nil, fmt.Errorf("%w: dependency %s does not exist", taskmodel.ErrInvalidLink, dep)
		}
	}
	graph[cardID] = deps
	if reaches(graph, deps, cardID) {
		return nil, fmt.Errorf("%w: these dependencies would make a cycle back to %s", taskmodel.ErrInvalidLink, cardID)
	}
	return deps, nil
}

// reaches reports whether target is reachable from start in graph.
func reaches(graph map[string][]string, start []string, target string) bool {
	seen := map[string]bool{}
	stack := slices.Clone(start)
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if n == target {
			return true
		}
		if seen[n] {
			continue
		}
		seen[n] = true
		stack = append(stack, graph[n]...)
	}
	return false
}
