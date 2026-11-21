# ADR-001 — Cards in SQLite, documents as Markdown files

**Status:** Accepted · **Date:** 2026-11-22 · **Decider:** the owner

## Context

Camelot replaces a hand-edited Markdown board (one bullet per card, no types, no history) and links cards to requirement documents that already live as Markdown files in two places (a private vault and a code repository). Agents and the owner write at the same time; the insights need a status history.

## Options

| | Files are the data (card files + an append-only events file) | SQLite is the data |
|---|---|---|
| Readable with Camelot down | yes | no (mitigated below) |
| Hand edits | possible; one bypassing the event log is detected by `check` | impossible; every change goes through the CLI, MCP or UI |
| Concurrent writers | last write wins per file | WAL + busy timeout serialise writes; stale reads refused |
| Dependency | none | `modernc.org/sqlite` (pure Go, CGO-free) |

## Decision

**Cards, their links and their history in SQLite** (the owner's choice over the files option). **Documents stay Markdown files** where they live, with a flat frontmatter block (`id`, `type`, `title`, `status`, `owner`, `updated`, `serves`) as the single source of their fields and their one upward link.

## Consequences

- One transaction per change: the card row and its history event commit together.
- The database lives in the vault (`.camelot/camelot.db`), never in this repository; a daily `VACUUM INTO` backup keeps 14 copies; a generated Markdown board (`export-board`) is the plain-text fallback.
- Schema by w-tools `migrationx`, one set (`task`), its CREATE migrations edited in place until the owner's board moves in (the cutover), new migrations after.
- Frontmatter is parsed by a strict in-repo parser (no YAML library); anything outside the subset is a reported problem.
