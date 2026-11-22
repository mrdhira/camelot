package cockpithandler

import (
	"net/http"
	"strings"

	"github.com/mrdhira/camelot/services/camelot/internal/searchmodel"
	"github.com/mrdhira/camelot/services/camelot/src/app"
)

// result is one palette entry as JSON.
type result struct {
	Kind   string `json:"kind"`
	ID     string `json:"id"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Href   string `json:"href"`
}

// pages are the palette's navigation entries.
var pages = []result{
	{Kind: "page", ID: "cockpit", Title: "Cockpit", Detail: "what's new, decisions, numbers", Href: "/"},
	{Kind: "page", ID: "board", Title: "Board", Detail: "all cards by stage", Href: "/board"},
	{Kind: "page", ID: "insights", Title: "Insights", Detail: "flow measures", Href: "/insights"},
	{Kind: "page", ID: "docs", Title: "Documents", Detail: "the document chain", Href: "/docs"},
	{Kind: "page", ID: "new", Title: "New card", Detail: "create a card", Href: "/tasks/new"},
	{Kind: "page", ID: "check", Title: "Data quality", Detail: "problems in cards and documents", Href: "/check"},
	{Kind: "page", ID: "design", Title: "Design book", Detail: "colours, badges and the rules", Href: "/design"},
}

// searchAll returns the pages whose title matches q, then the search service's cards and documents with their pages' links.
func (h *Handler) searchAll(r *http.Request, q string) ([]result, error) {
	found, err := h.searchSvc.Search(r.Context(), &searchmodel.Request{Query: q})
	if err != nil {
		return nil, err
	}
	out := []result{}
	lq := strings.ToLower(strings.TrimSpace(q))
	for _, p := range pages {
		if lq == "" || strings.Contains(strings.ToLower(p.Title), lq) {
			out = append(out, p)
		}
	}
	for _, f := range found {
		href := app.TaskHref(f.ID)
		if f.Kind == searchmodel.KindDoc {
			href = app.DocHref(f.ID)
		}
		out = append(out, result{Kind: f.Kind, ID: f.ID, Title: f.Title, Detail: f.Detail, Href: href})
	}
	return out, nil
}
