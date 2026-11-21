# CLAUDE.md

@AGENTS.md

## Claude-specific

- Recording work in Camelot itself (cards, stages, decisions): the vault skill `camelot`.
- The UI test stage includes a visual pass: view the suite's screenshots (`tests/playwright/test-results/visual/`) at 390 and 1280 px.
- Never run Playwright, or any test that writes, against the real vault database; `make e2e` uses a throwaway copy of `testdata/vault`.
