SHELL := /bin/bash
.DEFAULT_GOAL := help

SERVICE  := services/camelot
BIN      := bin/camelot
GO_FILES := ./...

# Tailwind standalone CLI, downloaded into bin/ and checked against its
# pinned sha256 on every run. The shadcn-templ theme and component sources
# come from the module pinned in go.mod, exposed to tailwind.css in
# $(SERVICE)/src/.shadcn-templ (gitignored): a link to the module and its
# style-nova.css cut to the components the pages use (CSS_KEEP).
TAILWIND  := v4.3.3
CSS_KEEP  := |Alert|Badge|Button|Card|Dialog|Empty|Field|Input|Label|Native Select|Progress|Sheet|Switch|Table|Textarea|Tooltip|
CSS_OUT   := $(SERVICE)/src/assets/css/app.css

# Third-party frontend files committed in src/assets, each pinned by sha256.
GEIST_WOFF2_SUM := 9ce2d97863cab59a0c7522534112d6c42a4e25c0e6b2ab37766761b0f82cfbd6
GEIST_OFL_SUM   := c683bfbcc7e087f5d37a54ef628f10387c451a83ddc459b151403a164ac46c90

GITLEAKS := v8.30.1

.PHONY: help
help: ## list the targets
	@grep -hE '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-14s %s\n", $$1, $$2}'

.PHONY: generate
generate: ## templ generate
	go tool templ generate -path $(SERVICE)/src

.PHONY: css
css: ## Tailwind $(TAILWIND): tailwind.css -> app.css (committed)
	@case "$$(uname -s)/$$(uname -m)" in \
		Darwin/arm64) p=macos-arm64 sum=cdf646702987a743464dff4d9c60fd4480d1c1e73dd819a9a67f1078815dce9d;; \
		Linux/x86_64) p=linux-x64 sum=dc61b3ac6b8c9ca874c0cc4c57b2409791a64c5540404ca5f5367360babc313a;; \
		Linux/aarch64|Linux/arm64) p=linux-arm64 sum=55fd0b241214eff3de1e8ee4f22796662f2d2e7a49bcfca7477cfd0bac398195;; \
		*) echo "css: no pinned Tailwind $(TAILWIND) for $$(uname -sm)"; exit 1;; \
	esac; \
	tw=bin/tailwindcss-$(TAILWIND)-$$p; \
	if [ ! -f "$$tw" ]; then mkdir -p bin && curl -fsSL -o "$$tw" \
		https://github.com/tailwindlabs/tailwindcss/releases/download/$(TAILWIND)/tailwindcss-$$p || { rm -f "$$tw"; exit 1; }; fi; \
	echo "$$sum  $$tw" | shasum -a 256 -c --status || { rm -f "$$tw"; echo "css: $$tw sha256 mismatch, removed"; exit 1; }; \
	chmod +x "$$tw" || exit 1; \
	go mod download github.com/axadrn/shadcn-templ/v2 || exit 1; \
	dir=$$(go list -m -f '{{.Dir}}' github.com/axadrn/shadcn-templ/v2) && [ -n "$$dir" ] || \
		{ echo "css: shadcn-templ module not found after go mod download"; exit 1; }; \
	gen=$(SERVICE)/src/.shadcn-templ; mkdir -p $$gen && ln -sfn "$$dir" $$gen/module && \
	awk -v keep='$(CSS_KEEP)' 'BEGIN { on = 1 } \
		/^  \/\* MARK: / { n = $$0; sub(/^  \/\* MARK: /, "", n); sub(/ \*\/$$/, "", n); on = index(keep, "|" n "|") > 0 } \
		/^}/ { on = 1 } on' "$$dir/assets/css/styles/style-nova.css" > $$gen/style-nova.css && \
	"$$tw" --input $(SERVICE)/src/assets/css/tailwind.css --output $(CSS_OUT) --minify

.PHONY: build
build: generate css ## build bin/camelot (CGO off)
	CGO_ENABLED=0 go build -trimpath -o $(BIN) ./$(SERVICE)

.PHONY: run
run: build ## serve natively on http://127.0.0.1:7700 for the vault two levels up (no Docker)
	CAMELOT_ACTOR=dhira $(BIN) --vault ../.. serve

.PHONY: install
install: build ## install camelot into GOBIN for agents (CLI and MCP)
	CGO_ENABLED=0 go install -trimpath ./$(SERVICE)

.PHONY: vendor-check
vendor-check: ## verify the committed third-party frontend files
	@printf '%s  %s\n%s  %s\n' \
		$(GEIST_WOFF2_SUM) $(SERVICE)/src/assets/fonts/geist-9ce2d97863cab59a.woff2 \
		$(GEIST_OFL_SUM) $(SERVICE)/src/assets/vendor/OFL-Geist.txt | shasum -a 256 -c --status \
		|| { echo "vendor-check: a pinned frontend file changed"; exit 1; }

.PHONY: check
check: ## quality gate: fmt, vet, lint, tidy, templ + css freshness, vendor pins, build, test -race, govulncheck
	@test -z "$$(gofmt -l services)" || { gofmt -l services; echo "gofmt: files above need formatting"; exit 1; }
	go vet $(GO_FILES)
	golangci-lint run $(GO_FILES)
	go mod tidy -diff
	@before=$$(find $(SERVICE)/src -name '*_templ.go' -exec shasum {} + | sort); go tool templ generate -path $(SERVICE)/src >/dev/null 2>&1; \
		after=$$(find $(SERVICE)/src -name '*_templ.go' -exec shasum {} + | sort); \
		[ "$$before" = "$$after" ] || { echo "templ: generated files were stale; regenerated, review and keep them"; exit 1; }
	@before=$$(shasum $(CSS_OUT)); $(MAKE) -s css >/dev/null; [ "$$before" = "$$(shasum $(CSS_OUT))" ] || \
		{ echo "css: app.css was stale; rebuilt, review and keep it"; exit 1; }
	@$(MAKE) -s vendor-check
	@! grep -rn 'templ.Raw' $(SERVICE)/src --include='*.templ' | grep -v 'templ.Raw(string(p.HTML))\|templ.Raw(string(p.Description))\|templ.Raw(string(html))' \
		|| { echo "templ.Raw outside the rendered-Markdown slots (markdown.HTML values)"; exit 1; }
	CGO_ENABLED=0 go build -trimpath -o $(BIN) ./$(SERVICE)
	go test -race $(GO_FILES)
	govulncheck $(GO_FILES)

.PHONY: e2e
e2e: build ## Playwright against a server on a throwaway copy of the fixture vault
	cd tests/playwright && pnpm install --frozen-lockfile && pnpm exec playwright test

.PHONY: scan
scan: ## secret scan (gitleaks $(GITLEAKS)): staged changes, the full history, and no absolute home paths; run before every push
	go run github.com/zricethezav/gitleaks/v8@$(GITLEAKS) git --pre-commit --staged --redact --no-banner .
	@if git rev-parse --verify -q HEAD >/dev/null; then go run github.com/zricethezav/gitleaks/v8@$(GITLEAKS) git --redact --no-banner .; fi
	@! git grep -nI --cached -E '/(Users|home)/[A-Za-z]' -- . ':!Makefile' | grep -v '/home/nonroot' || { echo "scan: an absolute home path is staged"; exit 1; }

# Docker stack (deployment/local): http://camelot.localhost in the Colima VM.
COLIMA_PROFILE ?= wigataintech
DOCKER  := docker --context colima-$(COLIMA_PROFILE)
COMPOSE := $(DOCKER) compose -f deployment/local/docker-compose.yaml
VAULT   := $(abspath ../..)

.PHONY: up
up: ## build the image and serve http://camelot.localhost (Colima profile $(COLIMA_PROFILE)); refused while a native camelot serve runs
	@! pgrep -fl 'camelot( .*)? serve' || { echo "up: a native camelot serve holds the database; stop it first (SQLite cannot be shared across the VM)"; exit 1; }
	mkdir -p $(VAULT)/.camelot
	CAMELOT_VAULT_DIR=$(VAULT) $(COMPOSE) up --build --detach --wait
	@echo "Camelot: http://camelot.localhost · agents: scripts/camelot (docker exec while the stack serves)"

.PHONY: down
down: ## stop the stack and release the database for the Mac's CLI
	CAMELOT_VAULT_DIR=$(VAULT) $(COMPOSE) down
	rm -f $(VAULT)/.camelot/server.lock

.PHONY: logs
logs: ## follow the stack's logs
	CAMELOT_VAULT_DIR=$(VAULT) $(COMPOSE) logs --follow
