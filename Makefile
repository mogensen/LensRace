SHELL := /bin/bash

BACKEND_PORT ?= 3000
FRONTEND_PORT ?= 5173
DB_PATH ?= lensrace.db
BINARY := bin/server

# Must match playwright.config.ts's webServer DB_PATH.
E2E_DB_PATH := /tmp/lensrace-playwright-e2e.db

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@echo "Usage: make <target>"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_-]+:.*##/ { printf "  %-16s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: install
install: ## Install backend and frontend dependencies
	go mod download
	cd frontend && pnpm install

.PHONY: dev
dev: ## Run backend and frontend dev servers together (Ctrl+C stops both)
	@echo "Backend:  http://localhost:$(BACKEND_PORT) (fiber dev if installed, else go run)"
	@echo "Frontend: http://localhost:$(FRONTEND_PORT)"
	@trap 'kill 0' EXIT; \
	PORT=$(BACKEND_PORT) DB_PATH=$(DB_PATH) bash -c 'command -v fiber >/dev/null 2>&1 && fiber dev || go run .' & \
	(cd frontend && pnpm dev) & \
	wait

.PHONY: stop
stop: ## Stop any running dev/combined servers on the configured ports, plus any tunnel from 'make public'
	-lsof -ti:$(BACKEND_PORT) | xargs kill 2>/dev/null
	-lsof -ti:$(FRONTEND_PORT) | xargs kill 2>/dev/null
	# Tunnel processes (started by 'make public') don't listen on either
	# port above — they connect *out* to the tunnel provider — so they
	# need to be matched and stopped separately.
	-pkill -f '^ngrok http' 2>/dev/null
	-pkill -f '^cloudflared tunnel' 2>/dev/null
	-pkill -f 'localtunnel --port' 2>/dev/null
	@echo "Stopped any processes listening on :$(BACKEND_PORT)/:$(FRONTEND_PORT) and any tunnel (ngrok/cloudflared/localtunnel)."

.PHONY: clean
clean: stop ## Remove build artifacts, dist folders, and the dev database
	rm -rf $(dir $(BINARY))
	rm -rf frontend/dist
	rm -rf frontend/public/vendor
	rm -rf frontend/node_modules/.cache
	rm -rf frontend/test-results frontend/playwright-report
	rm -f frontend/.eslintcache
	rm -f $(DB_PATH) $(DB_PATH)-journal
	@echo "Cleaned build artifacts and $(DB_PATH)."

.PHONY: build-frontend
build-frontend: ## Build the frontend (frontend/dist)
	cd frontend && pnpm build

.PHONY: build-backend
build-backend: ## Build a backend binary that serves the embedded frontend/dist
	CGO_ENABLED=0 go build -tags embed_frontend -o $(BINARY) .

.PHONY: build
build: build-frontend build-backend ## Build frontend + a single backend binary serving it (bin/server)
	@echo "Built $(BINARY) — serves the API and the frontend on one port."

.PHONY: run
run: build ## Build (see 'build') and run the combined binary
	PORT=$(BACKEND_PORT) DB_PATH=$(DB_PATH) ./$(BINARY)

.PHONY: public
public: build ## Run the combined build and expose it publicly (ngrok, else cloudflared, else localtunnel)
	@trap 'kill 0' EXIT; \
	PORT=$(BACKEND_PORT) DB_PATH=$(DB_PATH) ./$(BINARY) & \
	sleep 1; \
	echo "Serving http://localhost:$(BACKEND_PORT) — opening a public tunnel to it..."; \
	if command -v ngrok >/dev/null 2>&1; then \
		ngrok http $(BACKEND_PORT); \
	elif command -v cloudflared >/dev/null 2>&1; then \
		cloudflared tunnel --url http://localhost:$(BACKEND_PORT); \
	else \
		echo "No tunnel tool installed (checked ngrok, cloudflared) — falling back to 'npx localtunnel'."; \
		npx --yes localtunnel --port $(BACKEND_PORT); \
	fi

.PHONY: test
test: test-backend test-frontend ## Run backend + frontend tests

.PHONY: test-backend
test-backend: ## Run backend tests (go test ./... -race)
	go test ./... -race

.PHONY: test-frontend
test-frontend: ## Run frontend Playwright e2e tests (starts backend + frontend itself)
	# A forcibly-killed backend (e.g. an interrupted previous run) can leave
	# a hot journal on this file, which makes new connections stall trying
	# to recover it — starting from a clean file every run avoids that.
	rm -f $(E2E_DB_PATH) $(E2E_DB_PATH)-journal
	cd frontend && pnpm exec playwright test

.PHONY: lint
lint: ## Lint backend (gofmt + go vet) and frontend (oxlint + ESLint)
	test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)
	go vet ./...
	cd frontend && pnpm lint

.PHONY: fmt
fmt: ## Format backend (gofmt) and frontend (Prettier)
	gofmt -w .
	cd frontend && pnpm format
