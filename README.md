# Camelot

A local-first cockpit for one person running a team of AI agents: a Kanban board of cards, the documents those cards build (business, product and technical requirements, OKRs, KPIs), and the facts between them. The person reads and decides in the web UI; agents record their work through a CLI or an MCP server.

- **Cockpit:** what waits for your decision, what is in flight and for how long, what is blocked or stale, which agent changed what.
- **Board:** seven columns (Backlog, TODO, Plan, Work, QA, Review in Progress, Done), drag-and-drop, every move recorded with who and why.
- **Insights:** the Kanban Guide's flow measures (WIP, throughput, work item age, cycle time), open work by type and debt share, milestone progress; each number with its formula and sample size.
- **Documents:** Markdown files with frontmatter, linked in a chain (TRD → PRD → BRD → OKR), rendered for reading, with progress from the cards that build them and the gaps in the chain.

Cards and their history live in SQLite; documents stay Markdown files where they are. See `docs/adr/`.

## Run

Docker (Colima), on http://camelot.localhost:

```bash
make up      # image + deployment/local stack: the app behind its own Caddy on 127.0.0.1:80
make down    # stop; releases the database for the native CLI
```

Native, without Docker:

```bash
make build                         # bin/camelot (Go 1.27.4, CGO off)
bin/camelot --vault <dir> serve    # http://127.0.0.1:7700 (loopback only, no sign-in)
```

The Docker stack is its own compose project (`camelot-local`) and network, publishes only `127.0.0.1:80` and answers only `camelot.localhost` (any other Host: 421). SQLite's locks do not cross the Colima VM boundary, so while the container serves it owns the database (`.camelot/server.lock`): the native CLI refuses it, and agents go through `scripts/camelot`, which runs the command in the container (`docker exec`) when the stack is up and natively otherwise. See `docs/adr/ADR-003-docker-stack.md`.

The vault is a directory holding `tasks/TASK.md` (found by walking up from the working directory when `--vault` is not set); the database defaults to `<vault>/.camelot/camelot.db`, backed up daily to `.camelot/backups/` (14 kept).

## Agents

```bash
scripts/camelot task create --prefix DITA --title "…" --type feature --actor go-engineer
scripts/camelot task move --id DITA-012 --status wip --note "build started" --actor go-engineer
scripts/camelot task update --id DITA-012 --waiting "Pick A or B? A is cheaper." --actor dita
scripts/camelot insights            # JSON: every number with its formula
scripts/camelot check               # data quality; exit 1 on any problem
```

MCP (stdio; protocol 2025-11-25 with initialize, and 2026-07-28 with `server/discover`), through the wrapper so it works with or without the stack:

```json
{ "mcpServers": { "camelot": { "command": "<repo>/scripts/camelot", "args": ["mcp"], "env": { "CAMELOT_ACTOR": "dita" } } } }
```

Tools: `list_tasks`, `get_task`, `create_task`, `update_task`, `delete_task`, `list_docs`, `get_doc`, `set_doc_status`, `insights`, `check`.

## Develop

`make check` (the quality gate), `make e2e` (Playwright on a throwaway copy of `testdata/vault`), `make scan` (secret scan before a push). Rules: `AGENTS.md`.
