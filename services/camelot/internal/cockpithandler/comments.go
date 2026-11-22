package cockpithandler

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/mrdhira/camelot/services/camelot/internal/commentmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/docmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/taskmodel"
	"github.com/mrdhira/camelot/services/camelot/src/app"
)

// taskComment is POST /tasks/{id}/comments.
func (h *Handler) taskComment(w http.ResponseWriter, r *http.Request) {
	if !parseForm(w, r) {
		return
	}
	id := r.PathValue("id")
	c, err := h.commentSvc.Add(r.Context(), &commentmodel.AddRequest{Kind: commentmodel.KindCard, TargetID: id, Body: r.PostForm.Get("body"), Actor: Actor})
	if err != nil {
		if status, ok := userError(err); ok && status != http.StatusNotFound {
			h.renderTask(w, r, status, id, err.Error())
			return
		}
		h.fail(w, r, err)
		return
	}
	h.backToComment(w, r, c)
}

// docComment is POST /docs/{id}/comments.
func (h *Handler) docComment(w http.ResponseWriter, r *http.Request) {
	if !parseForm(w, r) {
		return
	}
	id := r.PathValue("id")
	c, err := h.commentSvc.Add(r.Context(), &commentmodel.AddRequest{Kind: commentmodel.KindDoc, TargetID: id, Section: r.PostForm.Get("section"), Body: r.PostForm.Get("body"), Actor: Actor})
	if err != nil {
		if status, ok := userError(err); ok && status != http.StatusNotFound {
			h.renderDoc(w, r, status, id, err.Error())
			return
		}
		h.fail(w, r, err)
		return
	}
	h.backToComment(w, r, c)
}

// commentResolve is POST /comments/{cid}/resolve.
func (h *Handler) commentResolve(w http.ResponseWriter, r *http.Request) {
	if !parseForm(w, r) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("cid"), 10, 64)
	if err != nil || id <= 0 {
		writePage(w, r, http.StatusNotFound, app.ErrorPage(http.StatusNotFound, "No such comment."))
		return
	}
	c, err := h.commentSvc.Resolve(r.Context(), &commentmodel.ResolveRequest{ID: id, Note: r.PostForm.Get("note"), Actor: Actor})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.backToComment(w, r, c)
}

// backToComment returns to the cockpit or board the form was on (the side
// panel), else to the comment on its card's or document's page.
func (h *Handler) backToComment(w http.ResponseWriter, r *http.Request, c *commentmodel.Comment) {
	if ref, err := url.Parse(r.Referer()); err == nil && ref.Host == r.Host && (ref.Path == "/" || ref.Path == "/board") {
		h.back(w, r, c.TargetID)
		return
	}
	anchor := "#camelot-comment-" + strconv.FormatInt(c.ID, 10)
	if c.Kind == commentmodel.KindDoc {
		id, err := docmodel.ParseID(c.TargetID)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		http.Redirect(w, r, app.DocHref(id)+anchor, http.StatusSeeOther) //nolint:gosec // a parsed document ID under /docs/
		return
	}
	id, err := taskmodel.ParseCardID(c.TargetID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	http.Redirect(w, r, app.TaskHref(id)+anchor, http.StatusSeeOther) //nolint:gosec // a parsed card ID under /tasks/
}

// parseForm reads a POST form, answering 400 when it cannot.
func parseForm(w http.ResponseWriter, r *http.Request) bool {
	if err := r.ParseForm(); err != nil {
		writePage(w, r, http.StatusBadRequest, app.ErrorPage(http.StatusBadRequest, "The form could not be read."))
		return false
	}
	return true
}
