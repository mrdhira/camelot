// Package cockpithandler serves Camelot's web UI: the cockpit, the board,
// cards, insights and documents, on a loopback listener.
package cockpithandler

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Wigata-Intech/w-tools/httpx"
	"github.com/Wigata-Intech/w-tools/httpx/middleware"

	"github.com/mrdhira/camelot/services/camelot/src/app"
)

// contentSecurityPolicy allows only our own scripts, styles, fonts and
// form targets; style attributes are allowed for the shadcn-templ
// components that set them (the progress bar's width).
const contentSecurityPolicy = "default-src 'none'; script-src 'self'; style-src 'self'; style-src-attr 'unsafe-inline'; " +
	"img-src 'self' data:; font-src 'self'; connect-src 'self'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'"

// maxFormBody bounds a form post.
const maxFormBody = 512 << 10

// NewServer returns the HTTP server on addr with recovery, a Host
// allow-list (any other Host gets 421, so a rebound DNS name never reaches
// the pages), request IDs, access logs, security headers, a body limit and
// cross-origin write protection; unmatched routes render the error page.
func NewServer(addr string, hosts []string, logger *slog.Logger) *httpx.Server {
	s := httpx.New(httpx.Config{
		Addr:         addr,
		WriteTimeout: 30 * time.Second,
		ErrorWriter: func(w http.ResponseWriter, r *http.Request, status int, _ string) {
			message := "There is no page here."
			if status == http.StatusMethodNotAllowed {
				message = "That action is not available here."
			}
			writePage(w, r, status, app.ErrorPage(status, message))
		},
	})
	s.Use(
		middleware.Recover(middleware.RecoverConfig{Log: logger, ErrorWriter: func(w http.ResponseWriter, r *http.Request, status int, _ string) {
			writePage(w, r, status, app.ErrorPage(status, "Something broke. The details are in the server log."))
		}}),
		allowHosts(hosts),
		middleware.RequestID(middleware.RequestIDConfig{}),
		middleware.Logger(middleware.LoggerConfig{Log: logger}),
		middleware.SecureHeaders(middleware.SecureHeadersConfig{ContentSecurityPolicy: contentSecurityPolicy, ReferrerPolicy: "same-origin"}),
		middleware.BodyLimit(middleware.BodyLimitConfig{Max: maxFormBody}),
		http.NewCrossOriginProtection().Handler,
	)
	return s
}

// allowHosts answers 421 Misdirected Request to a Host not in hosts (case-insensitive).
func allowHosts(hosts []string) httpx.Middleware {
	allowed := map[string]bool{}
	for _, h := range hosts {
		allowed[strings.ToLower(h)] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !allowed[strings.ToLower(r.Host)] {
				http.Error(w, "unknown host", http.StatusMisdirectedRequest)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
