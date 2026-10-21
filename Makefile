# Qadam — developer and operator commands. Run `make` for the list.
SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

COMPOSE := docker compose --env-file .env -f deploy/docker-compose.yml
SQLC_IMAGE := sqlc/sqlc:1.31.1
GO_TOOL := go tool -modfile=tools.mod
# The race detector needs cgo; set GO_TEST_FLAGS= on machines without a C compiler.
GO_TEST_FLAGS ?= -race -count=1

ifneq (,$(wildcard .env))
include .env
export
endif

.PHONY: help
help: ## Show this help
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) | \
	  awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.env:
	@echo "No .env file: run 'cp .env.example .env' first." >&2; exit 1

# ── Stack ──────────────────────────────────────────────────────────────────
.PHONY: dev down logs ps infra
dev: .env ## Build and start the whole stack in Docker (web on http://localhost:$WEB_PORT)
	$(COMPOSE) up -d --build
	$(COMPOSE) up -d --wait api worker web
	@echo ""
	@echo "  Qadam is up:   http://localhost:$(WEB_PORT)"
	@echo "  API:           http://localhost:$(API_PORT)/api/v1/meta"
	@echo "  Mail catcher:  http://localhost:$(MAILPIT_UI_PORT)"
	@echo "  MinIO console: http://localhost:$(MINIO_CONSOLE_PORT)"

down: ## Stop the stack (data volumes are kept)
	$(COMPOSE) down

logs: ## Follow logs of the application services
	$(COMPOSE) logs -f api worker web

ps: ## Show the state of every service
	$(COMPOSE) ps

infra: .env ## Start only infrastructure, for running api/worker/web natively
	$(COMPOSE) up -d --wait postgres valkey minio gotenberg mailpit

# ── Native development (after `make infra`) ────────────────────────────────
.PHONY: migrate migrate-down migrate-status run-api run-worker run-web
migrate: ## Apply database migrations
	cd backend && go run ./cmd/migrate up

migrate-down: ## Roll back the latest migration
	cd backend && go run ./cmd/migrate down

migrate-status: ## Show migration status
	cd backend && go run ./cmd/migrate status

run-api: ## Run the API from source
	cd backend && go run ./cmd/api

run-worker: ## Run the worker from source
	cd backend && go run ./cmd/worker

run-web: ## Run the Vite dev server (http://localhost:5173)
	cd frontend && npm run dev

# ── Code generation ────────────────────────────────────────────────────────
.PHONY: gen gen-api gen-sql gen-web
gen: gen-api gen-sql gen-web ## Regenerate code from openapi.yaml and SQL queries

gen-api:
	cd backend && $(GO_TOOL) oapi-codegen -config oapi-codegen.yaml api/openapi.yaml

gen-sql:
	docker run --rm -v "$(CURDIR)/backend:/src" -w /src $(SQLC_IMAGE) generate

gen-web:
	cd frontend && npm run gen:api

# ── Quality ────────────────────────────────────────────────────────────────
.PHONY: lint lint-backend lint-frontend test test-backend test-unit test-frontend e2e
lint: lint-backend lint-frontend ## Run every linter and type check

lint-backend:
	cd backend && $(GO_TOOL) golangci-lint run ./...

lint-frontend:
	cd frontend && npx tsc -b && npm run lint && npm run format:check && npm run i18n:check && npm run fonts:check

test: test-backend test-frontend ## Run unit and integration tests (integration tests need Docker)

test-backend:
	cd backend && go test $(GO_TEST_FLAGS) ./...

test-unit: ## Run only fast unit tests
	cd backend && go test -short ./...
	cd frontend && npm test

test-frontend:
	cd frontend && npm test

e2e: ## Run Playwright end-to-end tests against the running stack
	cd frontend && E2E_BASE_URL=http://localhost:$(WEB_PORT) npx playwright test

# ── Images ─────────────────────────────────────────────────────────────────
.PHONY: build
build: .env ## Build the api and web Docker images
	$(COMPOSE) build migrate web
