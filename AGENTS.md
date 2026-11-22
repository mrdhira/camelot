# AGENTS.md

Instructions for AI coding agents (and humans) working in this repository. Decisions behind these rules: `docs/adr/`.

## What this repo is

`camelot`: a local-first cockpit over cards (Kanban), documents (BRD, PRD, TRD, OKR, KPI) and the insights between them. One owner reads the web UI; agents write through the CLI or the MCP server. Everything runs on one laptop: natively the web server listens on loopback only; in Docker only Caddy publishes 127.0.0.1:80; it has no sign-in, and answers only its allow-listed Host names.

## Layout

```text
go.mod                              # the only module (ADR-002); tool templ
services/camelot/                   # one binary
  main.go
  cmd/                              # composition root: serve | task (list, show, create, update, move, delete) | doc (list, show, status) | comment (list, add, resolve) | insights | check | backup | import-board | export-board | mcp | probe; owner.go = the database's single-kernel guard (ADR-003)
  config/                           # flags, each also CAMELOT_<NAME>: vault, db, doc-roots, listen (loopback only unless behind-proxy), hosts (the Host allow-list), behind-proxy, actor, time-zone, log-level
  internal/
    taskmodel/ taskrepository/{repository.go, sqlite/ (+ migrations/)} taskservice/ (+ taskservicetest/)   # cards, links, history events (SQLite, ADR-001)
    docmodel/  docrepository/{repository.go, markdown/, sqlite/ (+ migrations/)}  docservice/              # documents: Markdown files with frontmatter, read through os.Root; their history (snapshots + events, compare-and-set Sync every 30 s in serve, first sync a silent baseline)
    commentmodel/ commentrepository/{repository.go, sqlite/ (+ migrations/)} commentservice/   # one comment thread per card and per document (a document comment may name a heading); append-only, a resolution is a row pointing at its comment; "awaits reply" = the newest comment is Dhira's and open
    feedmodel/ feedrepository/ feedservice/  # "what's new": card and document events after the seen marker (event IDs, never clocks); the change stamp live.js polls
    searchmodel/ searchservice/     # cards and documents for the ⌘K palette; Unicode-safe snippets
    insightmodel/ insightservice/   # flow measures, waiting, blocked, stale, document progress and gaps, data-quality check
    cockpithandler/                 # the web UI: server.go (hardening), handler.go (routes and pages), search.go, comments.go
    mcphandler/                     # MCP over stdio (2025-11-25 initialize and 2026-07-28 server/discover)
    pkg/                            # no app knowledge: frontmatter, markdown (goldmark, safe mode), sqlitedb, taskboard (legacy TASK.md parser)
  src/app/                          # templ pages (one package: layout with the ⌘K palette, theme toggle and side-panel sheet, cockpit, board, task + TaskPanel, insights, docs, comments, parts) + helpers.go, scripts.go (shadcn component scripts in bundle order), shell.go (header facts)
  src/assets/                       # assets.go (embedded, hashed names; shadcn scripts from the module), css/{tailwind.css, app.css (make css, committed)}, fonts/, vendor/, js/{theme,live,panel,palette,board}.js (theme.js runs in <head>: the light/dark switch; the system setting until switched)
  Dockerfile                        # scratch runtime, non-root; build context = repo root (.dockerignore allow-list)
deployment/local/                   # docker-compose.yaml (project camelot-local: camelot + Caddy on 127.0.0.1:80) and Caddyfile (http://camelot.localhost only)
scripts/camelot                     # agents' entry point: docker exec while the stack serves, else the native binary
testdata/vault/                     # a fictional vault for the Playwright suite
tests/playwright/                   # playwright-bdd features (pnpm)
docs/adr/
```

## Hard rules

1. **Dependencies.** Allowed: Go stdlib; `github.com/Wigata-Intech/w-tools/{cli,logger,httpx}` (use before writing your own); `github.com/a-h/templ`; `github.com/axadrn/shadcn-templ/v2` (components used as shipped, never copied); `modernc.org/sqlite` (ADR-001); `github.com/yuin/goldmark/v2` (Markdown, safe mode only). Test tooling via pnpm: `@playwright/test`, `playwright-bdd`, `@axe-core/playwright`. Anything else needs the owner's approval before it is written. Forbidden: web frameworks, ORMs, logging libraries other than slog, cobra/viper, testify, CDN-loaded assets.
2. **Layers.** `cmd → handler (cockpithandler, mcphandler) → service → repository interface ← repository implementation`; a service calls another domain's service, never its repository; `internal/pkg` imports no domain; nothing below the handlers imports `src/` or `cmd/` (depguard in `.golangci.yml`). Service methods take and return pointers to `<domain>model` request and response types; errors live in the owning model and are matched with `errors.Is`.
3. **One write, one event.** Every change to a card and its history event commit in one transaction. History is append-only. A stale write (the card's `updated_at` changed since it was read) is refused, never merged.
4. **Facts, not guesses.** Every number the UI or `insights` shows states its formula and sample size; an unknown value is shown as unknown (an imported card's age), never estimated. Refusals name the allowed values.
5. **One kernel per database.** SQLite's locks and WAL memory do not cross the Colima VM boundary: while the container serves, it owns the database (`server.lock`) and every other host's process refuses it; agents use `scripts/camelot`. Never open the vault's database from the Mac and the container at once.
6. **Files the app reads or writes** go through `os.Root` on the vault: no path leaves it, symlinks included. Document writes change only the frontmatter keys asked for, through a temporary file and a rename.
7. **UI.** Every page works without JavaScript; live.js never swaps a page while a field has focus or an unsaved value, a details block was toggled, a drag runs or a dialog is open. Stage colours always sit beside the stage's label; text never wears a series colour (a coloured dot beside it does). Two themes, one layout: light (refined) and dark (mission control), tokens in `tailwind.css`; a new colour gets a dark step validated on `#0d1218`; every page passes axe in both. templ only; never hand-edit `*_templ.go`. Scripts and styles from our origin only (CSP `script-src 'self'`; `style-src-attr 'unsafe-inline'` for the shipped components' style attributes). `templ.Raw` only for goldmark output (raw HTML omitted, dangerous links dropped); `make check` greps for any other use. Every page works without JavaScript; drag-and-drop is an enhancement over the card page's move form. English copy inline (single user).
8. **Agents' contract.** CLI values are flags only (no positional arguments); per-card environment variables are refused; output is JSON. MCP tools mirror the CLI; tool errors are results with `isError`, protocol errors are JSON-RPC errors. The browser's actor is always `dhira`; agents pass their own name.
9. **Secrets and personal data** never in git, logs or fixtures. `testdata/` is fictional. The database lives in the vault (`.camelot/`), never in this repo.
10. **Simplicity.** Minimum code for the task; match the neighbours; one-line godoc on exported identifiers; comments state contracts, never history.
11. **Version pins:** exact versions for Go modules, the Tailwind CLI and the vendored font (sha256 in the Makefile), and npm packages.

## Quality gate

```bash
make check    # gofmt · go vet · golangci-lint (depguard, gosec, errorlint, forbidigo) · go mod tidy -diff · templ and css freshness
              # · vendor pins · templ.Raw grep · CGO_ENABLED=0 build · go test -race · govulncheck
make e2e      # Playwright (playwright-bdd + axe, 390 and 1280 px) against bin/camelot on a fresh copy of testdata/vault
make scan     # gitleaks over the staged changes and the history, plus no absolute home paths: run before every push
make up/down  # the Docker stack on http://camelot.localhost (Colima profile COLIMA_PROFILE, default wigataintech)
```

Report the real output; never claim a check you did not run. Suppress a linter finding only with a targeted `//nolint:<linter> // <reason>`.

## Tests

Integration-first: services, handlers and the MCP server are tested through their real dependencies (in-memory SQLite with the migrations, a temporary vault with real files); no mocks of our own packages; time is controlled through the data (explicit event times), never a fake clock. Table-driven, blackbox `package <name>_test`, `-race` always.
