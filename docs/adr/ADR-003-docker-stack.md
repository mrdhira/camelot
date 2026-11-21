# ADR-003 — The Docker stack and one kernel per database

**Status:** Accepted · **Date:** 2026-11-22 · **Decider:** the owner

## Context

The owner wants Camelot in Docker behind Caddy at `camelot.localhost`, beside another local stack on the same Colima VM whose Caddy holds `127.0.0.1:443` with a CA name-constrained to its own domain. The cards live in SQLite inside the vault, which the container bind-mounts; agents on the Mac use the CLI and the MCP server. SQLite's file locks and its WAL shared memory work only between processes on one kernel: the Mac and the Colima VM are two.

## Decision

- Own compose project `camelot-local`, own network, own Caddy publishing only `127.0.0.1:80`; plain HTTP (`*.localhost` is loopback and a browser secure context), one site `camelot.localhost`, any other Host refused by Caddy and again by the app (421; DNS rebinding).
- The app container runs non-root on scratch, read-only, with only `tasks/`, `.camelot/` and the two document roots of the vault mounted (same paths under `/vault`): the rest of the vault is not in the container; `--behind-proxy` permits its non-loopback listen and requires the Host allow-list.
- **One kernel per database:** the serving container writes `.camelot/server.lock`; any process on another host refuses to open the database while it exists. Agents use `scripts/camelot`, which runs the command in the container (`docker exec`) while the stack serves and natively otherwise; `.mcp.json` points at it. A native MCP server that started before the stack re-checks the owner before every tool call. The stack has no restart policy: only `make up` starts it, and it refuses while a native `camelot serve` runs. `make down` removes a lock left by a crash.

## Consequences

- The Colima VM must run for the Docker UI; the native `make run` stays available and needs nothing.
- A process that bypasses both the wrapper and the lock (another tool opening the file) can still corrupt the database: the lock protects Camelot's own entry points, not arbitrary SQLite clients.
