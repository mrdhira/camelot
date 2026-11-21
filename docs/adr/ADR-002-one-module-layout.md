# ADR-002 — One Go module, the layered layout, boundaries by depguard

**Status:** Accepted · **Date:** 2026-11-22 · **Decider:** the owner

## Context

The owner's other monorepo grew 17 `go.mod` files, a `go.work` and `replace` lines; research for that repo found the Go project's guidance ("one repo = one module" unless parts are published separately) and that a binary links only what its `main` imports.

## Decision

- One `go.mod` at the root. Services under `services/<name>/`, shared code (when a second service needs it) under `packages/`.
- Each service: `cmd/` (composition root), `config/`, `internal/<domain>model|repository|service`, one handler package per transport (`cockpithandler` for the web, `mcphandler` for agents), `internal/pkg/` without app knowledge, `src/` for templ pages and assets.
- Boundaries the module system no longer gives are depguard rules in `.golangci.yml` (domains and transports never import `src/` or `cmd/`; `internal/pkg` imports no domain), checked by `make check`.

## Consequences

Adding a package is creating a directory. One `go test ./...`, one lint run, one tidy. A shared package published later gets its own module at that time.
