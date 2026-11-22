package cockpithandler

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/Wigata-Intech/w-tools/httpx"
	"github.com/a-h/templ"

	"github.com/mrdhira/camelot/services/camelot/internal/commentmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/commentservice"
	"github.com/mrdhira/camelot/services/camelot/internal/docmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/docservice"
	"github.com/mrdhira/camelot/services/camelot/internal/feedmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/feedservice"
	"github.com/mrdhira/camelot/services/camelot/internal/insightmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/insightservice"
	"github.com/mrdhira/camelot/services/camelot/internal/pkg/markdown"
	"github.com/mrdhira/camelot/services/camelot/internal/searchservice"
	"github.com/mrdhira/camelot/services/camelot/internal/taskmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/taskservice"
	"github.com/mrdhira/camelot/services/camelot/src/app"
	"github.com/mrdhira/camelot/services/camelot/src/assets"
)

// Actor is who the browser's changes are recorded as.
const Actor = commentmodel.Owner

// doneLimit is how many Done cards the board shows unless asked for all.
const doneLimit = 20

// Config is the handler's settings.
type Config struct {
	// DocRoots are the document roots, shown on the documents page.
	DocRoots []string
}

// Handler serves the pages.
type Handler struct {
	config     Config
	logger     *slog.Logger
	taskSvc    taskservice.Service
	docSvc     docservice.Service
	insightSvc insightservice.Service
	feedSvc    feedservice.Service
	searchSvc  searchservice.Service
	commentSvc commentservice.Service
}

// New returns the handler.
func New(config Config, logger *slog.Logger, taskSvc taskservice.Service, docSvc docservice.Service, insightSvc insightservice.Service, feedSvc feedservice.Service, searchSvc searchservice.Service, commentSvc commentservice.Service) *Handler {
	return &Handler{config: config, logger: logger, taskSvc: taskSvc, docSvc: docSvc, insightSvc: insightSvc, feedSvc: feedSvc, searchSvc: searchSvc, commentSvc: commentSvc}
}

// Routes registers every page on s; pages get the header's facts (shell).
func (h *Handler) Routes(s *httpx.Server) {
	s.Get("/assets/", assets.Serve)
	s.Get("/healthz", healthz)
	s.Get("/changes/stamp", h.stamp)
	s.Get("/search", h.search)
	s.Post("/feed/seen", h.feedSeen)
	s.Post("/tasks/{id}/answer", h.taskAnswer)
	s.Get("/tasks/{id}/panel", h.taskPanel)
	s.Get("/{$}", h.cockpit)
	s.Get("/board", h.board)
	s.Post("/board", h.moveFallback)
	s.Get("/insights", h.insights)
	s.Get("/check", h.check)
	s.Get("/tasks/new", h.taskNew)
	s.Post("/tasks", h.taskCreate)
	s.Get("/tasks/{id}", h.task)
	s.Post("/tasks/{id}", h.taskUpdate)
	s.Get("/tasks/{id}/edit", h.taskEdit)
	s.Post("/tasks/{id}/move", h.taskMove)
	s.Get("/tasks/{id}/delete", h.taskDeleteForm)
	s.Post("/tasks/{id}/delete", h.taskDelete)
	s.Get("/tasks/{id}/files/{name}", h.taskFile)
	s.Get("/docs", h.docs)
	s.Get("/docs/{id}", h.doc)
	s.Post("/docs/{id}/status", h.docStatus)
	s.Post("/tasks/{id}/comments", h.taskComment)
	s.Post("/docs/{id}/comments", h.docComment)
	s.Post("/comments/{cid}/resolve", h.commentResolve)
}

// healthz is GET /healthz: the process serves.
func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte("ok\n"))
}

// cockpit is GET /: what's new first, then the decisions, then the numbers.
func (h *Handler) cockpit(w http.ResponseWriter, r *http.Request) {
	feed, err := h.feedSvc.Feed(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	o, err := h.insightSvc.Overview(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.writePageWith(w, r, http.StatusOK, app.Cockpit(feed, o), feed)
}

// stamp is GET /changes/stamp: a value that changes with any card or document change (live.js).
func (h *Handler) stamp(w http.ResponseWriter, r *http.Request) {
	stamp, err := h.feedSvc.Stamp(r.Context())
	if err != nil {
		h.logger.ErrorContext(r.Context(), "stamp failed", "error", err)
		http.Error(w, "stamp failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(stamp.String()))
}

// feedSeen is POST /feed/seen: marks the feed seen up to the as_of the page showed.
func (h *Handler) feedSeen(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writePage(w, r, http.StatusBadRequest, app.ErrorPage(http.StatusBadRequest, "The form could not be read."))
		return
	}
	if err := h.feedSvc.MarkSeen(r.Context(), &feedmodel.SeenRequest{Stamp: r.PostForm.Get("stamp")}); err != nil {
		if errors.Is(err, feedmodel.ErrInvalid) {
			h.writePage(w, r, http.StatusBadRequest, app.ErrorPage(http.StatusBadRequest, err.Error()))
			return
		}
		h.fail(w, r, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// taskAnswer is POST /tasks/{id}/answer: Dhira's decision on the waiting question.
func (h *Handler) taskAnswer(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writePage(w, r, http.StatusBadRequest, app.ErrorPage(http.StatusBadRequest, "The form could not be read."))
		return
	}
	id := r.PathValue("id")
	answer := r.PostForm.Get("answer")
	if custom := strings.TrimSpace(r.PostForm.Get("custom")); custom != "" && answer == "" {
		answer = custom
	}
	_, err := h.taskSvc.Answer(r.Context(), &taskmodel.AnswerRequest{ID: id, Answer: answer, Actor: Actor, UpdatedAt: parseTime(r.PostForm.Get("updated_at"))})
	if err != nil {
		if status, ok := userError(err); ok {
			h.renderTask(w, r, status, id, err.Error())
			return
		}
		h.fail(w, r, err)
		return
	}
	h.back(w, r, id)
}

// taskPanel is GET /tasks/{id}/panel: the card as a fragment for the board's side panel.
func (h *Handler) taskPanel(w http.ResponseWriter, r *http.Request) {
	props, err := h.taskProps(r, r.PathValue("id"), "")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	_ = httpx.Render(w, r, http.StatusOK, app.TaskPanel(props))
}

// search is GET /search?q=: cards, documents (title and text) and pages for the ⌘K palette, as JSON.
func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	results, err := h.searchAll(r, r.URL.Query().Get("q"))
	if err != nil {
		h.logger.ErrorContext(r.Context(), "search failed", "error", err)
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusOK, results)
}

// board is GET /board: the columns, filtered by the query.
func (h *Handler) board(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := app.BoardFilter{Type: q.Get("type"), Milestone: q.Get("milestone"), Query: q.Get("q"), Waiting: q.Get("waiting") != "", AllDone: q.Get("all_done") != "",
		Blocked: q.Get("blocked") != "", Stale: q.Get("stale") != "", Status: q.Get("status")}
	cf := taskmodel.CardFilter{MilestoneID: filter.Milestone, Query: filter.Query, Waiting: filter.Waiting}
	if filter.Status != "" {
		st, err := taskmodel.ParseStatus(filter.Status)
		if err != nil {
			h.writePage(w, r, http.StatusBadRequest, app.ErrorPage(http.StatusBadRequest, err.Error()))
			return
		}
		cf.Statuses = []taskmodel.Status{st}
	}
	if filter.Type != "" {
		t, err := taskmodel.ParseType(filter.Type)
		if err != nil {
			writePage(w, r, http.StatusBadRequest, app.ErrorPage(http.StatusBadRequest, err.Error()))
			return
		}
		cf.Types = []taskmodel.Type{t}
	}
	ctx := r.Context()
	cards, err := h.taskSvc.List(ctx, &taskmodel.ListRequest{Filter: cf})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	all, err := h.taskSvc.List(ctx, nil)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if filter.Blocked || filter.Stale {
		o, err := h.insightSvc.Overview(ctx)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		keep := map[string]bool{}
		if filter.Blocked {
			for _, b := range o.Blocked {
				keep[b.Card.ID] = true
			}
		}
		if filter.Stale {
			for _, a := range o.Stale {
				keep[a.Card.ID] = true
			}
		}
		cards = slices.DeleteFunc(cards, func(c *taskmodel.Card) bool { return !keep[c.ID] })
	}
	status := map[string]taskmodel.Status{}
	var milestones []*taskmodel.Card
	for _, c := range all {
		status[c.ID] = c.Status
		if c.Type == taskmodel.TypeMilestone {
			milestones = append(milestones, c)
		}
	}
	summaries, err := h.commentSvc.Summaries(r.Context(), commentmodel.KindCard)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	p := app.BoardProps{Filter: filter, Milestones: milestones, DoneLimit: doneLimit}
	for _, st := range taskmodel.Statuses {
		col := app.BoardColumn{Status: st}
		var done []*taskmodel.Card
		for _, c := range cards {
			if c.Status != st || (filter.Type == "" && c.Type == taskmodel.TypeMilestone) {
				continue
			}
			if st == taskmodel.StatusDone {
				done = append(done, c)
				continue
			}
			col.Cards = append(col.Cards, boardCard(c, status, summaries))
		}
		slices.SortStableFunc(done, func(a, b *taskmodel.Card) int { return b.UpdatedAt.Compare(a.UpdatedAt) })
		for i, c := range done {
			if i >= doneLimit && !filter.AllDone {
				col.Hidden = len(done) - doneLimit
				break
			}
			col.Cards = append(col.Cards, boardCard(c, status, summaries))
		}
		p.Total += len(col.Cards) + col.Hidden
		p.Columns = append(p.Columns, col)
	}
	h.writePage(w, r, http.StatusOK, app.Board(p))
}

// boardCard flags a card blocked when a dependency is not Done, with its open comments.
func boardCard(c *taskmodel.Card, status map[string]taskmodel.Status, comments map[string]commentmodel.Summary) app.BoardCard {
	bc := app.BoardCard{Card: c, Comments: comments[c.ID]}
	if c.Status != taskmodel.StatusDone {
		for _, d := range c.Deps {
			if status[d] != taskmodel.StatusDone {
				bc.Blocked = true
			}
		}
	}
	return bc
}

// insights is GET /insights.
func (h *Handler) insights(w http.ResponseWriter, r *http.Request) {
	o, err := h.insightSvc.Overview(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.writePage(w, r, http.StatusOK, app.Insights(o))
}

// check is GET /check.
func (h *Handler) check(w http.ResponseWriter, r *http.Request) {
	c, err := h.insightSvc.Check(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.writePage(w, r, http.StatusOK, app.Check(c))
}

// task is GET /tasks/{id}.
func (h *Handler) task(w http.ResponseWriter, r *http.Request) {
	h.renderTask(w, r, http.StatusOK, r.PathValue("id"), "")
}

// renderTask writes a card's page with an optional error.
func (h *Handler) renderTask(w http.ResponseWriter, r *http.Request, status int, id, errMsg string) {
	props, err := h.taskProps(r, id, errMsg)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.writePage(w, r, status, app.Task(props))
}

// taskProps reads a card with its rendered description and folder files.
func (h *Handler) taskProps(r *http.Request, id, errMsg string) (app.TaskProps, error) {
	detail, err := h.taskSvc.Get(r.Context(), &taskmodel.GetRequest{ID: id})
	if err != nil {
		return app.TaskProps{}, err
	}
	description, err := markdown.Render([]byte(detail.Card.Description))
	if err != nil {
		return app.TaskProps{}, err
	}
	files, err := h.taskSvc.Files(r.Context(), &taskmodel.FilesRequest{ID: id})
	if err != nil && !errors.Is(err, taskmodel.ErrFolderMissing) {
		return app.TaskProps{}, err
	}
	thread, err := h.commentSvc.Thread(r.Context(), &commentmodel.ListRequest{Kind: commentmodel.KindCard, TargetID: detail.Card.ID})
	if err != nil {
		return app.TaskProps{}, err
	}
	return app.TaskProps{
		Detail:      detail,
		Description: description,
		Files:       files,
		Thread:      thread,
		Error:       errMsg,
		Now:         time.Now().UTC(),
	}, nil
}

// taskFile is GET /tasks/{id}/files/{name}: one Markdown file of the card's folder.
func (h *Handler) taskFile(w http.ResponseWriter, r *http.Request) {
	file, err := h.taskSvc.File(r.Context(), &taskmodel.FileRequest{ID: r.PathValue("id"), Name: r.PathValue("name")})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	html, err := markdown.Render(file.Content)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.writePage(w, r, http.StatusOK, app.TaskFile(file.Card, file.Name, html))
}

// taskNew is GET /tasks/new.
func (h *Handler) taskNew(w http.ResponseWriter, r *http.Request) {
	p := app.TaskFormProps{New: true, Prefix: "DITA", Type: string(taskmodel.TypeTask), Status: string(taskmodel.StatusBacklog)}
	h.renderForm(w, r, http.StatusOK, p)
}

// renderForm writes the card form with the milestones to choose from.
func (h *Handler) renderForm(w http.ResponseWriter, r *http.Request, status int, p app.TaskFormProps) {
	milestones, err := h.taskSvc.List(r.Context(), &taskmodel.ListRequest{Filter: taskmodel.CardFilter{Types: []taskmodel.Type{taskmodel.TypeMilestone}}})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	p.Milestones = milestones
	h.writePage(w, r, status, app.TaskForm(p))
}

// formProps reads the card form.
func formProps(r *http.Request) app.TaskFormProps {
	f := r.PostForm
	return app.TaskFormProps{
		ID:        strings.TrimSpace(f.Get("id")),
		Prefix:    strings.TrimSpace(f.Get("prefix")),
		Title:     f.Get("title"),
		Type:      f.Get("type"),
		Status:    f.Get("status"),
		Milestone: f.Get("milestone"),
		Assignee:  f.Get("assignee"),
		Waiting:   f.Get("waiting"),
		Options:   f.Get("options"),
		Folder:    f.Get("folder"),
		Deps:      f.Get("deps"),
		Docs:      f.Get("docs"),
		Desc:      f.Get("description"),
		Note:      f.Get("note"),
		UpdatedAt: f.Get("updated_at"),
	}
}

// taskCreate is POST /tasks.
func (h *Handler) taskCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writePage(w, r, http.StatusBadRequest, app.ErrorPage(http.StatusBadRequest, "The form could not be read."))
		return
	}
	p := formProps(r)
	p.New = true
	prefix := p.Prefix
	if p.ID != "" {
		prefix = ""
	}
	c, err := h.taskSvc.Create(r.Context(), &taskmodel.CreateRequest{
		ID:          p.ID,
		Prefix:      prefix,
		MilestoneID: p.Milestone,
		Title:       p.Title,
		Type:        p.Type,
		Status:      p.Status,
		Assignee:    p.Assignee,
		Waiting:     p.Waiting,
		Options:     splitLines(p.Options),
		Folder:      p.Folder,
		Description: p.Desc,
		Deps:        splitList(p.Deps),
		Docs:        splitList(p.Docs),
		Actor:       Actor,
		Note:        p.Note,
	})
	if err != nil {
		if status, ok := userError(err); ok {
			p.Error = err.Error()
			h.renderForm(w, r, status, p)
			return
		}
		h.fail(w, r, err)
		return
	}
	http.Redirect(w, r, app.TaskHref(c.ID), http.StatusSeeOther)
}

// taskEdit is GET /tasks/{id}/edit.
func (h *Handler) taskEdit(w http.ResponseWriter, r *http.Request) {
	detail, err := h.taskSvc.Get(r.Context(), &taskmodel.GetRequest{ID: r.PathValue("id")})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	c := detail.Card
	h.renderForm(w, r, http.StatusOK, app.TaskFormProps{
		ID:        c.ID,
		Title:     c.Title,
		Type:      string(c.Type),
		Status:    string(c.Status),
		Milestone: c.MilestoneID,
		Assignee:  c.Assignee,
		Waiting:   c.Waiting,
		Options:   strings.Join(c.Options, "\n"),
		Folder:    c.Folder,
		Deps:      strings.Join(c.Deps, ", "),
		Docs:      strings.Join(c.Docs, ", "),
		Desc:      c.Description,
		UpdatedAt: app.ISO(c.UpdatedAt),
	})
}

// taskUpdate is POST /tasks/{id}: every field of the form; unchanged fields record nothing.
func (h *Handler) taskUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writePage(w, r, http.StatusBadRequest, app.ErrorPage(http.StatusBadRequest, "The form could not be read."))
		return
	}
	id := r.PathValue("id")
	p := formProps(r)
	p.ID = id
	deps, docs, options := splitList(p.Deps), splitList(p.Docs), splitLines(p.Options)
	request := &taskmodel.UpdateRequest{
		ID:          id,
		MilestoneID: &p.Milestone,
		Title:       &p.Title,
		Type:        &p.Type,
		Status:      &p.Status,
		Assignee:    &p.Assignee,
		Waiting:     &p.Waiting,
		Options:     &options,
		Folder:      &p.Folder,
		Description: &p.Desc,
		Deps:        &deps,
		Docs:        &docs,
		Actor:       Actor,
		Note:        p.Note,
		UpdatedAt:   parseTime(p.UpdatedAt),
	}
	_, err := h.taskSvc.Update(r.Context(), request)
	if err != nil && !errors.Is(err, taskmodel.ErrNothingToChange) {
		if status, ok := userError(err); ok {
			p.Error = err.Error()
			h.renderForm(w, r, status, p)
			return
		}
		h.fail(w, r, err)
		return
	}
	h.redirectToTask(w, r, id)
}

// taskMove is POST /tasks/{id}/move; return=board goes back to the board.
func (h *Handler) taskMove(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writePage(w, r, http.StatusBadRequest, app.ErrorPage(http.StatusBadRequest, "The form could not be read."))
		return
	}
	id := r.PathValue("id")
	status := r.PostForm.Get("status")
	h.update(w, r, &taskmodel.UpdateRequest{ID: id, Status: &status, Actor: Actor, Note: r.PostForm.Get("note"), UpdatedAt: parseTime(r.PostForm.Get("updated_at"))}, r.PostForm.Get("return") == "board")
}

// moveFallback answers the board's move form posted without a card (no script ran).
func (h *Handler) moveFallback(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/board", http.StatusSeeOther)
}

// update applies a one-field change and returns to the card or the board;
// a refused change re-renders the card with the reason.
func (h *Handler) update(w http.ResponseWriter, r *http.Request, request *taskmodel.UpdateRequest, toBoard bool) {
	_, err := h.taskSvc.Update(r.Context(), request)
	if err != nil && !errors.Is(err, taskmodel.ErrNothingToChange) {
		if status, ok := userError(err); ok {
			h.renderTask(w, r, status, request.ID, err.Error())
			return
		}
		h.fail(w, r, err)
		return
	}
	if !toBoard {
		h.back(w, r, request.ID)
		return
	}
	target := "/board"
	if ref, err := url.Parse(r.Referer()); err == nil && ref.Path == "/board" && ref.Host == r.Host {
		target = "/board?" + ref.Query().Encode()
	}
	http.Redirect(w, r, target, http.StatusSeeOther) //nolint:gosec // always /board on this host, with the board's own query
}

// back returns the browser to the page it came from on this host (the
// cockpit, the board, a card), else to the card's page.
func (h *Handler) back(w http.ResponseWriter, r *http.Request, id string) {
	if ref, err := url.Parse(r.Referer()); err == nil && ref.Host == r.Host && (ref.Path == "/" || ref.Path == "/board") {
		target := ref.Path
		if ref.RawQuery != "" {
			target += "?" + ref.Query().Encode()
		}
		http.Redirect(w, r, target, http.StatusSeeOther) //nolint:gosec // one of this host's own pages, its query re-encoded
		return
	}
	h.redirectToTask(w, r, id)
}

// redirectToTask sends the browser to a card's page; the ID is re-parsed,
// so the target is always a /tasks/ path on this host.
func (h *Handler) redirectToTask(w http.ResponseWriter, r *http.Request, rawID string) {
	id, err := taskmodel.ParseCardID(rawID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	http.Redirect(w, r, app.TaskHref(id), http.StatusSeeOther) //nolint:gosec // a parsed card ID under /tasks/
}

// taskDeleteForm is GET /tasks/{id}/delete.
func (h *Handler) taskDeleteForm(w http.ResponseWriter, r *http.Request) {
	detail, err := h.taskSvc.Get(r.Context(), &taskmodel.GetRequest{ID: r.PathValue("id")})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.writePage(w, r, http.StatusOK, app.TaskDelete(detail.Card, ""))
}

// taskDelete is POST /tasks/{id}/delete.
func (h *Handler) taskDelete(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writePage(w, r, http.StatusBadRequest, app.ErrorPage(http.StatusBadRequest, "The form could not be read."))
		return
	}
	id := r.PathValue("id")
	err := h.taskSvc.Delete(r.Context(), &taskmodel.DeleteRequest{ID: id, Actor: Actor, Note: r.PostForm.Get("note")})
	if err != nil {
		if status, ok := userError(err); ok && !errors.Is(err, taskmodel.ErrNotFound) {
			detail, gerr := h.taskSvc.Get(r.Context(), &taskmodel.GetRequest{ID: id})
			if gerr != nil {
				h.fail(w, r, gerr)
				return
			}
			h.writePage(w, r, status, app.TaskDelete(detail.Card, err.Error()))
			return
		}
		h.fail(w, r, err)
		return
	}
	http.Redirect(w, r, "/board", http.StatusSeeOther)
}

// docs is GET /docs.
func (h *Handler) docs(w http.ResponseWriter, r *http.Request) {
	o, err := h.insightSvc.Docs(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	summaries, err := h.commentSvc.Summaries(r.Context(), commentmodel.KindDoc)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.writePage(w, r, http.StatusOK, app.Docs(app.DocsProps{Overview: o, Tree: docTree(o.Docs, summaries), Roots: h.config.DocRoots}))
}

// docTree arranges documents from the top of the chain down; a document
// serving several shows under each, and one serving nothing known is a root.
func docTree(docs []insightmodel.DocProgress, comments map[string]commentmodel.Summary) []app.DocNode {
	byID := map[string]insightmodel.DocProgress{}
	children := map[string][]string{}
	for _, d := range docs {
		byID[d.Doc.ID] = d
	}
	var roots []string
	for _, d := range docs {
		linked := false
		for _, up := range d.Doc.Serves {
			if _, ok := byID[up]; ok {
				children[up] = append(children[up], d.Doc.ID)
				linked = true
			}
		}
		if !linked {
			roots = append(roots, d.Doc.ID)
		}
	}
	var build func(id string, seen map[string]bool) app.DocNode
	build = func(id string, seen map[string]bool) app.DocNode {
		n := app.DocNode{Progress: byID[id], Comments: comments[id].Open}
		seen[id] = true
		for _, c := range children[id] {
			if !seen[c] {
				n.Children = append(n.Children, build(c, seen))
			}
		}
		delete(seen, id)
		return n
	}
	var tree []app.DocNode
	for _, id := range roots {
		tree = append(tree, build(id, map[string]bool{}))
	}
	return tree
}

// doc is GET /docs/{id}.
func (h *Handler) doc(w http.ResponseWriter, r *http.Request) {
	h.renderDoc(w, r, http.StatusOK, r.PathValue("id"), "")
}

// renderDoc writes a document's page with an optional error.
func (h *Handler) renderDoc(w http.ResponseWriter, r *http.Request, status int, id, errMsg string) {
	ctx := r.Context()
	chain, err := h.docSvc.Get(ctx, &docmodel.GetRequest{ID: id})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	overview, err := h.insightSvc.Docs(ctx)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var progress insightmodel.DocProgress
	for _, d := range overview.Docs {
		if d.Doc.ID == chain.Doc.ID {
			progress = d
		}
	}
	ids := []string{chain.Doc.ID}
	for _, d := range chain.Down {
		ids = append(ids, d.ID)
	}
	var cards []*taskmodel.Card
	seen := map[string]bool{}
	for _, docID := range ids {
		linked, err := h.taskSvc.List(ctx, &taskmodel.ListRequest{Filter: taskmodel.CardFilter{DocID: docID}})
		if err != nil {
			h.fail(w, r, err)
			return
		}
		for _, c := range linked {
			if !seen[c.ID] {
				seen[c.ID] = true
				cards = append(cards, c)
			}
		}
	}
	html, err := markdown.Render(chain.Doc.Body)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	events, err := h.docSvc.Events(ctx, &docmodel.EventFilter{DocID: chain.Doc.ID, Limit: 10})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	thread, err := h.commentSvc.Thread(ctx, &commentmodel.ListRequest{Kind: commentmodel.KindDoc, TargetID: chain.Doc.ID})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.writePage(w, r, status, app.Doc(app.DocProps{
		Thread:   thread,
		Chain:    chain,
		Progress: progress,
		Cards:    cards,
		HTML:     html,
		TOC:      markdown.Headings(html),
		Events:   events,
		Now:      time.Now().UTC(),
		Error:    errMsg,
	}))
}

// docStatus is POST /docs/{id}/status.
func (h *Handler) docStatus(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writePage(w, r, http.StatusBadRequest, app.ErrorPage(http.StatusBadRequest, "The form could not be read."))
		return
	}
	id := r.PathValue("id")
	if _, err := h.docSvc.SetStatus(r.Context(), &docmodel.SetStatusRequest{ID: id, Status: r.PostForm.Get("status"), Actor: Actor}); err != nil {
		if status, ok := userError(err); ok && !errors.Is(err, docmodel.ErrNotFound) {
			h.renderDoc(w, r, status, id, err.Error())
			return
		}
		h.fail(w, r, err)
		return
	}
	docID, err := docmodel.ParseID(id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if ref, err := url.Parse(r.Referer()); err == nil && ref.Host == r.Host && ref.Path == "/" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, app.DocHref(docID), http.StatusSeeOther) //nolint:gosec // a parsed document ID under /docs/
}

// userError maps a refusal the user can fix to its status.
func userError(err error) (int, bool) {
	switch {
	case errors.Is(err, taskmodel.ErrNotFound), errors.Is(err, docmodel.ErrNotFound), errors.Is(err, taskmodel.ErrFileNotFound), errors.Is(err, commentmodel.ErrNotFound):
		return http.StatusNotFound, true
	case errors.Is(err, taskmodel.ErrStale), errors.Is(err, taskmodel.ErrExists):
		return http.StatusConflict, true
	case errors.Is(err, taskmodel.ErrNothingWaiting):
		return http.StatusUnprocessableEntity, true
	case errors.Is(err, taskmodel.ErrInvalidID), errors.Is(err, taskmodel.ErrInvalidStatus), errors.Is(err, taskmodel.ErrInvalidType),
		errors.Is(err, taskmodel.ErrInvalidField), errors.Is(err, taskmodel.ErrInvalidLink), errors.Is(err, docmodel.ErrInvalid),
		errors.Is(err, commentmodel.ErrInvalid):
		return http.StatusUnprocessableEntity, true
	}
	return 0, false
}

// fail writes the error page: a refusal with its message, anything else a logged 500.
func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	if status, ok := userError(err); ok {
		writePage(w, r, status, app.ErrorPage(status, err.Error()))
		return
	}
	h.logger.ErrorContext(r.Context(), "request failed", "error", err)
	writePage(w, r, http.StatusInternalServerError, app.ErrorPage(http.StatusInternalServerError, "Something broke. The details are in the server log."))
}

// writePage renders a full page with the header's facts: the feed's unread
// count and the change stamp; a failure to read them leaves them empty.
func (h *Handler) writePage(w http.ResponseWriter, r *http.Request, status int, page templ.Component) {
	feed, err := h.feedSvc.Feed(r.Context())
	if err != nil {
		h.logger.WarnContext(r.Context(), "feed for the header failed", "error", err)
	}
	h.writePageWith(w, r, status, page, feed)
}

// writePageWith renders a full page with the header's facts taken from feed (nil leaves them empty).
func (h *Handler) writePageWith(w http.ResponseWriter, r *http.Request, status int, page templ.Component, feed *feedmodel.Feed) {
	shell := app.Shell{AsOf: time.Now().UTC()}
	if feed != nil {
		shell.Unread, shell.Stamp, shell.AsOf = len(feed.Items), feed.Stamp.String(), feed.AsOf
	}
	writePage(w, r.WithContext(app.WithShell(r.Context(), shell)), status, page)
}

// writePage renders a page, never cached.
func writePage(w http.ResponseWriter, r *http.Request, status int, page templ.Component) {
	w.Header().Set("Cache-Control", "no-store")
	_ = httpx.Render(w, r, status, page)
}

// parseTime reads an RFC 3339 time from a form; zero when absent or malformed.
func parseTime(v string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, v)
	if err != nil {
		return time.Time{}
	}
	return t
}

// splitList splits a comma-separated list, dropping blanks.
func splitList(v string) []string {
	out := []string{}
	for part := range strings.SplitSeq(v, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// splitLines splits text into its non-blank lines, trimmed.
func splitLines(v string) []string {
	out := []string{}
	for line := range strings.SplitSeq(strings.ReplaceAll(v, "\r\n", "\n"), "\n") {
		if l := strings.TrimSpace(line); l != "" {
			out = append(out, l)
		}
	}
	return out
}
