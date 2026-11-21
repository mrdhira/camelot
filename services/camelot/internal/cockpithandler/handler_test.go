package cockpithandler_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mrdhira/camelot/services/camelot/internal/cockpithandler"
	"github.com/mrdhira/camelot/services/camelot/internal/insightservice"
	"github.com/mrdhira/camelot/services/camelot/internal/taskmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/taskservice/taskservicetest"
	"github.com/mrdhira/camelot/services/camelot/src/app"
)

// site is the full handler on the harness's services.
type site struct {
	h   *taskservicetest.Harness
	srv http.Handler
}

func newSite(t *testing.T) *site {
	t.Helper()
	h := taskservicetest.New(t)
	h.Seed(t)
	folder := filepath.Join(h.Vault, "tasks", "DITA-001-seed")
	if err := os.MkdirAll(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "PLAN.md"), []byte("# Plan\n\n<script>alert(1)</script>\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.Vault, "tasks", "secret.md"), []byte("outside the folder"), 0o600); err != nil {
		t.Fatal(err)
	}
	folderName := "tasks/DITA-001-seed"
	if _, err := h.Tasks.Update(context.Background(), &taskmodel.UpdateRequest{ID: "DITA-001", Folder: &folderName, Actor: "dita"}); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := cockpithandler.NewServer("127.0.0.1:7700", []string{"127.0.0.1:7700", "camelot.localhost"}, logger)
	cockpithandler.New(cockpithandler.Config{DocRoots: []string{taskservicetest.DocRoot}}, logger, h.Tasks, h.Docs,
		insightservice.New(h.Tasks, h.Docs)).Routes(srv)
	return &site{h: h, srv: srv}
}

// do sends a request; form values make it a same-origin form POST.
func (s *site) do(t *testing.T, method, path string, form url.Values, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	r := httptest.NewRequest(method, "http://127.0.0.1:7700"+path, body)
	if form != nil {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Sec-Fetch-Site", "same-origin")
	}
	for k, v := range header {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	s.srv.ServeHTTP(w, r)
	return w
}

func TestPages(t *testing.T) {
	s := newSite(t)
	tests := []struct {
		name     string
		path     string
		status   int
		contains string
	}{
		{name: "cockpit", path: "/", status: http.StatusOK, contains: "Waiting for your decision"},
		{name: "board", path: "/board", status: http.StatusOK, contains: `data-card="DITA-001"`},
		{name: "board filtered to milestones", path: "/board?type=milestone", status: http.StatusOK, contains: `data-card="M1"`},
		{name: "insights", path: "/insights", status: http.StatusOK, contains: "Throughput per week"},
		{name: "documents", path: "/docs", status: http.StatusOK, contains: "PRD001"},
		{name: "a document rendered", path: "/docs/PRD001", status: http.StatusOK, contains: "<table>"},
		{name: "a card", path: "/tasks/DITA-001", status: http.StatusOK, contains: "Seed card"},
		{name: "a card folder file, raw HTML omitted", path: "/tasks/DITA-001/files/PLAN.md", status: http.StatusOK, contains: "<h1 id=\"plan\">Plan</h1>"},
		{name: "the create form", path: "/tasks/new", status: http.StatusOK, contains: `name="prefix"`},
		{name: "data quality", path: "/check", status: http.StatusOK, contains: "Data quality"},
		{name: "an unknown type filter", path: "/board?type=story", status: http.StatusBadRequest, contains: "invalid type"},
		{name: "a missing card", path: "/tasks/NOPE-1", status: http.StatusNotFound, contains: "card not found"},
		{name: "a missing document", path: "/docs/PRD404", status: http.StatusNotFound, contains: "document not found"},
		{name: "a file outside the card folder", path: "/tasks/DITA-001/files/..%2Fsecret.md", status: http.StatusNotFound, contains: "no such file"},
		{name: "an unknown page", path: "/nope", status: http.StatusNotFound, contains: "There is no page here."},
		{name: "health", path: "/healthz", status: http.StatusOK, contains: "ok"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := s.do(t, http.MethodGet, tt.path, nil, nil)
			if w.Code != tt.status {
				t.Fatalf("status = %d, want %d", w.Code, tt.status)
			}
			body := w.Body.String()
			if !strings.Contains(body, tt.contains) {
				t.Errorf("body lacks %q", tt.contains)
			}
			if strings.Contains(body, "<script>alert") {
				t.Errorf("raw HTML from Markdown reached the page")
			}
			if csp := w.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self'") {
				t.Errorf("Content-Security-Policy = %q", csp)
			}
			if w.Header().Get("Cache-Control") != "no-store" && tt.status == http.StatusOK {
				t.Errorf("Cache-Control = %q, want no-store", w.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestHosts(t *testing.T) {
	s := newSite(t)
	tests := []struct {
		name     string
		host     string
		expected int
	}{
		{name: "the listen address", host: "127.0.0.1:7700", expected: http.StatusOK},
		{name: "the proxy's name, any case", host: "Camelot.Localhost", expected: http.StatusOK},
		{name: "a rebound DNS name", host: "evil.example:7700", expected: http.StatusMisdirectedRequest},
		{name: "the right name on another port", host: "camelot.localhost:8080", expected: http.StatusMisdirectedRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Host = tt.host
			w := httptest.NewRecorder()
			s.srv.ServeHTTP(w, r)
			if w.Code != tt.expected {
				t.Errorf("status = %d, want %d", w.Code, tt.expected)
			}
			if tt.expected != http.StatusOK && strings.Contains(w.Body.String(), "Seed card") {
				t.Errorf("a refused host saw the cards")
			}
		})
	}
}

func TestWrites(t *testing.T) {
	type expected struct {
		status   int
		location string
		contains string
		check    func(t *testing.T, s *site)
	}
	statusOf := func(id string) func(t *testing.T, s *site) taskmodel.Status {
		return func(t *testing.T, s *site) taskmodel.Status {
			got, err := s.h.Tasks.Get(context.Background(), &taskmodel.GetRequest{ID: id})
			if err != nil {
				t.Fatal(err)
			}
			return got.Card.Status
		}
	}
	tests := []struct {
		name     string
		path     string
		form     func(s *site) url.Values
		header   map[string]string
		expected expected
	}{
		{
			name: "create with a prefix, recorded as dhira",
			path: "/tasks",
			form: func(*site) url.Values {
				return url.Values{"prefix": {"DITA"}, "title": {"From the UI"}, "type": {"bug"}, "status": {"todo"}, "docs": {"PRD001"}}
			},
			expected: expected{status: http.StatusSeeOther, location: "/tasks/DITA-002", check: func(t *testing.T, s *site) {
				got, err := s.h.Tasks.Get(context.Background(), &taskmodel.GetRequest{ID: "DITA-002"})
				if err != nil || got.Events[0].Actor != cockpithandler.Actor || got.Card.Type != taskmodel.TypeBug {
					t.Errorf("created card = %+v, %v", got, err)
				}
			}},
		},
		{
			name:     "create refused: the form comes back with the reason",
			path:     "/tasks",
			form:     func(*site) url.Values { return url.Values{"prefix": {"DITA"}, "title": {""}} },
			expected: expected{status: http.StatusUnprocessableEntity, contains: "a title is one line"},
		},
		{
			name: "move from the board returns to the board",
			path: "/tasks/DITA-001/move",
			form: func(s *site) url.Values {
				got, _ := s.h.Tasks.Get(context.Background(), &taskmodel.GetRequest{ID: "DITA-001"})
				return url.Values{"status": {"plan"}, "updated_at": {app.ISO(got.Card.UpdatedAt)}, "return": {"board"}}
			},
			header: map[string]string{"Referer": "http://127.0.0.1:7700/board?q=seed"},
			expected: expected{status: http.StatusSeeOther, location: "/board?q=seed", check: func(t *testing.T, s *site) {
				if got := statusOf("DITA-001")(t, s); got != taskmodel.StatusPlan {
					t.Errorf("status = %s, want plan", got)
				}
			}},
		},
		{
			name: "a stale move is refused with the reason",
			path: "/tasks/DITA-001/move",
			form: func(*site) url.Values {
				return url.Values{"status": {"wip"}, "updated_at": {app.ISO(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))}}
			},
			expected: expected{status: http.StatusConflict, contains: "changed since it was read", check: func(t *testing.T, s *site) {
				if got := statusOf("DITA-001")(t, s); got != taskmodel.StatusBacklog {
					t.Errorf("status = %s, want unchanged backlog", got)
				}
			}},
		},
		{
			name:     "a cross-origin post is refused",
			path:     "/tasks/DITA-001/move",
			form:     func(*site) url.Values { return url.Values{"status": {"done"}} },
			header:   map[string]string{"Sec-Fetch-Site": "cross-site"},
			expected: expected{status: http.StatusForbidden},
		},
		{
			name: "deleting a milestone with cards is refused",
			path: "/tasks/M1/delete",
			form: func(*site) url.Values { return url.Values{"note": {"test"}} },
			expected: expected{status: http.StatusUnprocessableEntity, contains: "unlink them first", check: func(t *testing.T, s *site) {
				if got := statusOf("M1")(t, s); got != taskmodel.StatusBacklog {
					t.Errorf("M1 changed: %s", got)
				}
			}},
		},
		{
			name:     "a document status from the review form",
			path:     "/docs/PRD001/status",
			form:     func(*site) url.Values { return url.Values{"status": {"in-review"}} },
			expected: expected{status: http.StatusSeeOther, location: "/docs/PRD001"},
		},
		{
			name:     "an unknown document status",
			path:     "/docs/PRD001/status",
			form:     func(*site) url.Values { return url.Values{"status": {"done"}} },
			expected: expected{status: http.StatusUnprocessableEntity, contains: "allowed: draft"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newSite(t)
			w := s.do(t, http.MethodPost, tt.path, tt.form(s), tt.header)
			if w.Code != tt.expected.status {
				t.Fatalf("status = %d, want %d; body %.300s", w.Code, tt.expected.status, w.Body.String())
			}
			if tt.expected.location != "" && w.Header().Get("Location") != tt.expected.location {
				t.Errorf("Location = %q, want %q", w.Header().Get("Location"), tt.expected.location)
			}
			if tt.expected.contains != "" && !strings.Contains(w.Body.String(), tt.expected.contains) {
				t.Errorf("body lacks %q", tt.expected.contains)
			}
			if tt.expected.check != nil {
				tt.expected.check(t, s)
			}
		})
	}
}
