// Package insightservice computes the cockpit's numbers from the cards,
// their history and the documents.
package insightservice

import (
	"context"
	"errors"
	"slices"
	"sort"
	"time"

	"github.com/mrdhira/camelot/services/camelot/internal/docmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/docservice"
	"github.com/mrdhira/camelot/services/camelot/internal/insightmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/taskmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/taskservice"
)

// Service is the insight service.
type Service interface {
	// Overview returns the task flow measures and what waits for Dhira, as of now.
	Overview(ctx context.Context) (*insightmodel.Overview, error)
	// Docs returns every document with its progress and gaps.
	Docs(ctx context.Context) (*insightmodel.DocOverview, error)
	// Check returns every data-quality problem in cards, links and documents.
	Check(ctx context.Context) (*insightmodel.Check, error)
}

type service struct {
	taskSvc taskservice.Service
	docSvc  docservice.Service
}

// New returns the Service.
func New(taskSvc taskservice.Service, docSvc docservice.Service) Service {
	return &service{taskSvc: taskSvc, docSvc: docSvc}
}

// history is what the events say about one card.
type history struct {
	started  time.Time
	finished time.Time
	undated  bool
	last     *taskmodel.Event
}

// Overview computes every task number over all live cards and events.
func (s *service) Overview(ctx context.Context) (*insightmodel.Overview, error) {
	now := time.Now().UTC()
	cards, err := s.taskSvc.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	events, err := s.taskSvc.Events(ctx, nil)
	if err != nil {
		return nil, err
	}
	docs, err := s.docSvc.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	hist := histories(events)
	o := &insightmodel.Overview{Now: now, Problems: len(docs.Problems)}
	byID := map[string]*taskmodel.Card{}
	statusCount := map[taskmodel.Status]int{}
	openCount := map[taskmodel.Type]int{}
	var cycles []time.Duration
	cyclesByType := map[taskmodel.Type][]time.Duration{}
	for _, c := range cards {
		byID[c.ID] = c
	}
	for _, c := range cards {
		if c.Type == taskmodel.TypeMilestone {
			continue
		}
		o.Cards++
		statusCount[c.Status]++
		h := hist[c.ID]
		if c.Status != taskmodel.StatusDone {
			openCount[c.Type]++
			o.OpenTotal++
			if c.Type.IsDebt() {
				o.OpenDebt++
			}
		}
		if c.Status.Started() {
			o.WIP++
			aged := insightmodel.AgedCard{Card: c, Known: !h.started.IsZero(), LastEvent: h.last}
			if aged.Known {
				aged.Age = now.Sub(h.started)
			}
			o.InFlight = append(o.InFlight, aged)
			if h.last != nil && now.Sub(h.last.CreatedAt) > insightmodel.StaleAfter {
				o.Stale = append(o.Stale, aged)
			}
		}
		if c.Status == taskmodel.StatusDone && h.undated && h.finished.IsZero() {
			o.UndatedDone++
		}
		if c.Status == taskmodel.StatusDone && !h.started.IsZero() && !h.finished.IsZero() && h.finished.After(h.started) {
			d := h.finished.Sub(h.started)
			cycles = append(cycles, d)
			cyclesByType[c.Type] = append(cyclesByType[c.Type], d)
		}
		if c.Waiting != "" {
			o.Waiting = append(o.Waiting, c)
		}
		if c.Status == taskmodel.StatusReview {
			o.InReview = append(o.InReview, c)
		}
		if c.Status != taskmodel.StatusDone {
			var by []string
			for _, dep := range c.Deps {
				if d, ok := byID[dep]; !ok || d.Status != taskmodel.StatusDone {
					by = append(by, dep)
				}
			}
			if len(by) > 0 {
				o.Blocked = append(o.Blocked, insightmodel.BlockedCard{Card: c, By: by})
			}
		}
	}
	for _, st := range taskmodel.Statuses {
		o.ByStatus = append(o.ByStatus, insightmodel.StatusCount{Status: st, Count: statusCount[st]})
	}
	for _, t := range taskmodel.Types {
		if t != taskmodel.TypeMilestone {
			o.Open = append(o.Open, insightmodel.TypeCount{Type: t, Count: openCount[t]})
		}
	}
	o.Cycle = percentiles(cycles)
	for _, t := range taskmodel.Types {
		if len(cyclesByType[t]) > 0 {
			o.CycleByType = append(o.CycleByType, insightmodel.TypeCycle{Type: t, Cycle: percentiles(cyclesByType[t])})
		}
	}
	threshold := insightmodel.AgeFallback
	if o.Cycle.Shown() {
		threshold = o.Cycle.P85
	}
	for i := range o.InFlight {
		o.InFlight[i].Flagged = o.InFlight[i].Known && o.InFlight[i].Age > threshold
	}
	slices.SortFunc(o.InFlight, func(a, b insightmodel.AgedCard) int { return int(b.Age - a.Age) })
	o.Throughput, o.Finished = throughput(cards, hist, now)
	o.Milestones = milestones(cards)
	o.Agents = agents(events, now)
	if len(events) > 15 {
		o.Recent = events[:15]
	} else {
		o.Recent = events
	}
	for _, d := range docs.Docs {
		if d.Status == docmodel.StatusInReview {
			o.DocsInReview = append(o.DocsInReview, d)
		}
	}
	return o, nil
}

// Docs computes each document's progress through its chain and its gaps.
func (s *service) Docs(ctx context.Context) (*insightmodel.DocOverview, error) {
	now := time.Now().UTC()
	list, err := s.docSvc.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	cards, err := s.taskSvc.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	events, err := s.taskSvc.Events(ctx, &taskmodel.EventFilter{Since: now.Add(-insightmodel.DocStaleAfter)})
	if err != nil {
		return nil, err
	}
	recent := map[string]bool{}
	for _, e := range events {
		recent[e.CardID] = true
	}
	cardsByDoc := map[string][]*taskmodel.Card{}
	for _, c := range cards {
		for _, d := range c.Docs {
			cardsByDoc[d] = append(cardsByDoc[d], c)
		}
	}
	children := map[string][]string{}
	for _, d := range list.Docs {
		for _, up := range d.Serves {
			children[up] = append(children[up], d.ID)
		}
	}
	o := &insightmodel.DocOverview{Problems: list.Problems, Counts: map[docmodel.Type]map[docmodel.Status]int{}}
	for _, t := range docmodel.Types {
		o.Counts[t] = map[docmodel.Status]int{}
	}
	for _, d := range list.Docs {
		o.Counts[d.Type][d.Status]++
		p := insightmodel.DocProgress{Doc: d, Direct: len(cardsByDoc[d.ID])}
		seenCard, seenDoc := map[string]bool{}, map[string]bool{}
		moving := false
		queue := []string{d.ID}
		for len(queue) > 0 {
			id := queue[0]
			queue = queue[1:]
			if seenDoc[id] {
				continue
			}
			seenDoc[id] = true
			for _, c := range cardsByDoc[id] {
				if seenCard[c.ID] || c.Type == taskmodel.TypeMilestone {
					continue
				}
				seenCard[c.ID] = true
				p.Total++
				if c.Status == taskmodel.StatusDone {
					p.Done++
				}
				moving = moving || recent[c.ID]
			}
			queue = append(queue, children[id]...)
		}
		p.Gaps = gaps(d, p, moving, now)
		o.Gaps += len(p.Gaps)
		o.Docs = append(o.Docs, p)
	}
	return o, nil
}

// Check lists card links to missing documents, missing card folders,
// waiting flags on Done cards and every document problem.
func (s *service) Check(ctx context.Context) (*insightmodel.Check, error) {
	cards, err := s.taskSvc.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	list, err := s.docSvc.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	docIDs := map[string]bool{}
	for _, d := range list.Docs {
		docIDs[d.ID] = true
	}
	check := &insightmodel.Check{Problems: []insightmodel.CheckProblem{}}
	for _, c := range cards {
		for _, d := range c.Docs {
			if !docIDs[d] {
				check.Problems = append(check.Problems, insightmodel.CheckProblem{Subject: c.ID, Message: "linked document " + d + " does not exist"})
			}
		}
		if c.Folder != "" {
			if _, err := s.taskSvc.Files(ctx, &taskmodel.FilesRequest{ID: c.ID}); errors.Is(err, taskmodel.ErrFolderMissing) {
				check.Problems = append(check.Problems, insightmodel.CheckProblem{Subject: c.ID, Message: "folder " + c.Folder + " does not exist"})
			} else if err != nil {
				return nil, err
			}
		}
		if c.Waiting != "" && c.Status == taskmodel.StatusDone {
			check.Problems = append(check.Problems, insightmodel.CheckProblem{Subject: c.ID, Message: "Done but still waiting on Dhira: " + c.Waiting})
		}
	}
	for _, p := range list.Problems {
		subject := p.Path
		if p.DocID != "" {
			subject = p.DocID + " (" + p.Path + ")"
		}
		check.Problems = append(check.Problems, insightmodel.CheckProblem{Subject: subject, Message: p.Message})
	}
	return check, nil
}

// histories reads, per card, the first move into a started status, the
// last move into Done and the newest event. Imported events count only
// as a Done date.
func histories(events []*taskmodel.Event) map[string]history {
	out := map[string]history{}
	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]
		h := out[e.CardID]
		h.last = e
		to := taskmodel.Status(e.ToValue)
		switch e.Field {
		case taskmodel.FieldCreated, taskmodel.FieldStatus:
			if to.Started() && h.started.IsZero() {
				h.started = e.CreatedAt
			}
			if to == taskmodel.StatusDone {
				h.finished = e.CreatedAt
			}
		case taskmodel.FieldImported:
			if to == taskmodel.StatusDone && e.Note != taskmodel.NoteImportedUndated {
				h.finished = e.CreatedAt
			}
			h.undated = to == taskmodel.StatusDone && e.Note == taskmodel.NoteImportedUndated
		}
		out[e.CardID] = h
	}
	return out
}

// throughput counts Done cards per week over the last ThroughputWeeks weeks.
func throughput(cards []*taskmodel.Card, hist map[string]history, now time.Time) ([]insightmodel.WeekCount, int) {
	start := weekStart(now).AddDate(0, 0, -7*(insightmodel.ThroughputWeeks-1))
	weeks := make([]insightmodel.WeekCount, insightmodel.ThroughputWeeks)
	for i := range weeks {
		weeks[i].Start = start.AddDate(0, 0, 7*i)
	}
	total := 0
	for _, c := range cards {
		f := hist[c.ID].finished
		if c.Status != taskmodel.StatusDone || c.Type == taskmodel.TypeMilestone || f.Before(start) {
			continue
		}
		i := int(f.Sub(start) / (7 * 24 * time.Hour))
		if i >= 0 && i < len(weeks) {
			weeks[i].Count++
			total++
		}
	}
	return weeks, total
}

// milestones counts each milestone's cards.
func milestones(cards []*taskmodel.Card) []insightmodel.MilestoneProgress {
	var out []insightmodel.MilestoneProgress
	for _, m := range cards {
		if m.Type != taskmodel.TypeMilestone {
			continue
		}
		p := insightmodel.MilestoneProgress{Card: m}
		for _, c := range cards {
			if c.MilestoneID == m.ID {
				p.Total++
				if c.Status == taskmodel.StatusDone {
					p.Done++
				}
			}
		}
		out = append(out, p)
	}
	return out
}

// agents groups the events in the activity window by actor, busiest first.
func agents(events []*taskmodel.Event, now time.Time) []insightmodel.AgentActivity {
	byActor := map[string]*insightmodel.AgentActivity{}
	for _, e := range events {
		if now.Sub(e.CreatedAt) > insightmodel.ActivityWindow {
			continue
		}
		a, ok := byActor[e.Actor]
		if !ok {
			a = &insightmodel.AgentActivity{Actor: e.Actor, LastAt: e.CreatedAt, LastCard: e.CardID, LastNote: e.Note}
			byActor[e.Actor] = a
		}
		a.Events++
	}
	out := make([]insightmodel.AgentActivity, 0, len(byActor))
	for _, a := range byActor {
		out = append(out, *a)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Events != out[j].Events {
			return out[i].Events > out[j].Events
		}
		return out[i].Actor < out[j].Actor
	})
	return out
}

// gaps lists what is missing or inconsistent around one document.
func gaps(d *docmodel.Doc, p insightmodel.DocProgress, moving bool, now time.Time) []string {
	var out []string
	if len(d.Serves) == 0 {
		switch d.Type {
		case docmodel.TypePRD:
			out = append(out, "no BRD: the PRD serves no business case")
		case docmodel.TypeTRD:
			out = append(out, "no PRD: the TRD serves no product requirement")
		case docmodel.TypeBRD, docmodel.TypeKPI:
			out = append(out, "serves no OKR")
		}
	}
	if (d.Type == docmodel.TypePRD || d.Type == docmodel.TypeTRD) && p.Total == 0 && d.Status != docmodel.StatusSuperseded {
		out = append(out, "no cards build it")
	}
	if d.Status == docmodel.StatusDraft && p.Total > 0 && p.Done == p.Total {
		out = append(out, "still a draft while all its cards are Done")
	}
	if updated, err := time.Parse(docmodel.DateLayout, d.Updated); err != nil {
		out = append(out, "no valid updated date")
	} else if moving && now.Sub(updated) > insightmodel.DocStaleAfter {
		out = append(out, "not updated for over 30 days while its cards move")
	}
	return out
}

// percentiles returns the nearest-rank 50th and 85th percentiles of
// sample, or only N when it is smaller than MinSample.
func percentiles(sample []time.Duration) insightmodel.Percentiles {
	p := insightmodel.Percentiles{N: len(sample)}
	if len(sample) < insightmodel.MinSample {
		return p
	}
	sorted := slices.Clone(sample)
	slices.Sort(sorted)
	rank := func(q int) time.Duration {
		i := (q*len(sorted) + 99) / 100
		return sorted[max(i, 1)-1]
	}
	p.P50, p.P85 = rank(50), rank(85)
	return p
}

// weekStart returns Monday 00:00 UTC of t's week.
func weekStart(t time.Time) time.Time {
	t = t.UTC()
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	offset := (int(day.Weekday()) + 6) % 7
	return day.AddDate(0, 0, -offset)
}
