# Qadam — developer and operator commands. Run `make` for the list.
SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

# The base file publishes only the web client; the development file adds the
# infrastructure ports on 127.0.0.1 and Mailpit (deploy/docker-compose.dev.yml).
COMPOSE_BASE := docker compose --env-file .env -f deploy/docker-compose.yml
COMPOSE := $(COMPOSE_BASE) -f deploy/docker-compose.dev.yml
SQLC_IMAGE := sqlc/sqlc:1.31.1
GO_TOOL := go tool -modfile=tools.mod
# The race detector needs cgo; set GO_TEST_FLAGS= on machines without a C compiler.
GO_TEST_FLAGS ?= -race -count=1

# Development commands see .env as Make variables too. Production commands
# (prod-*) leave .env to Docker Compose: Make would expand "$" in passwords.
ifeq (,$(filter prod-%,$(MAKECMDGOALS)))
ifneq (,$(wildcard .env))
include .env
export
endif
endif

.PHONY: help
help: ## Show this help
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) | \
	  awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.env:
	@echo "No .env file: on a server run 'make prod-init'; for development 'cp .env.example .env'." >&2; exit 1

# ── Stack ──────────────────────────────────────────────────────────────────
.PHONY: dev down logs ps infra seed compose-check
dev: .env ## Build and start the whole development stack in Docker (web on http://localhost:$WEB_PORT)
	$(COMPOSE) up -d --build
	$(COMPOSE) up -d --wait api worker web
	@echo ""
	@echo "  Qadam is up:   http://localhost:$(WEB_PORT)"
	@echo "  API:           http://localhost:$(API_PORT)/api/v1/meta"
	@echo "  Mail catcher:  http://localhost:$(MAILPIT_UI_PORT)"
	@echo "  MinIO console: http://localhost:$(MINIO_CONSOLE_PORT)"
	@echo "  Demo LDAP:     ldap://localhost:$(LDAP_PORT)"

down: ## Stop the stack (data volumes are kept)
	$(COMPOSE) down

logs: ## Follow logs of the application services
	$(COMPOSE) logs -f api worker web

ps: ## Show the state of every service
	$(COMPOSE) ps

infra: .env ## Start only infrastructure, for running api/worker/web natively
	$(COMPOSE) up -d --wait postgres valkey minio gotenberg mailpit openldap
	$(COMPOSE) run --rm db-roles
	$(COMPOSE) run --rm minio-init

seed: .env ## Load demo data into the running stack (KazUTB, demo accounts)
	$(COMPOSE) run --rm --no-deps worker /app/seed

compose-check: .env ## Validate the base compose file alone, with the development file and with the production file
	$(COMPOSE_BASE) config --quiet
	$(COMPOSE) config --quiet
	QADAM_DOMAIN=qadam.example.kz CADDY_TLS=internal $(COMPOSE_BASE) -f deploy/docker-compose.prod.yml config --quiet

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
.PHONY: lint lint-backend lint-frontend test test-backend test-unit test-frontend e2e e2e-clean ai-eval
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

ai-eval: ## Check the evaluation prompt on golden projects (QADAM_AI_* chooses the model; mock by default)
	cd backend && go run ./cmd/ai-eval

e2e: ## Run Playwright end-to-end tests against the running stack (cleans up after itself)
	cd frontend && E2E_BASE_URL=http://localhost:$(WEB_PORT) npx playwright test

e2e-clean: .env ## Remove what end-to-end runs left in the development database (e2e.* accounts and their projects)
	$(COMPOSE) run --rm --no-deps worker /app/seed clean-e2e

# ── Production (docs/OPERATIONS.md) ──────────────────────────────────────────
# Caddy terminates HTTPS in front of the base stack (deploy/docker-compose.prod.yml).
COMPOSE_PROD := $(COMPOSE_BASE) -f deploy/docker-compose.prod.yml
PROD_BUILD_ARGS := COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo unknown) BUILT_AT=$$(date -u +%Y-%m-%dT%H:%M:%SZ)

.PHONY: prod-init prod-up prod-down prod-ps prod-logs prod-bootstrap prod-demo prod-backup prod-update
prod-init: ## Write the production .env: address, HTTPS, mail and generated secrets
	deploy/scripts/prod-init.sh

prod-up: .env ## Build and start the production stack behind HTTPS (migrations apply on start)
	$(PROD_BUILD_ARGS) $(COMPOSE_PROD) up -d --build --remove-orphans
	$(COMPOSE_PROD) up -d --wait caddy api worker web
	@echo ""
	@echo "  Qadam is up: $$(grep -E '^PUBLIC_URL=' .env | cut -d= -f2-)"

prod-down: .env ## Stop the production stack (data volumes are kept)
	$(COMPOSE_PROD) down

prod-ps: .env ## Show the state of the production services
	$(COMPOSE_PROD) ps -a

prod-logs: .env ## Follow the logs of the production application services
	$(COMPOSE_PROD) logs -f --tail=200 caddy api worker web

prod-bootstrap: .env ## Create KazUTB and its first administrator: make prod-bootstrap ADMIN_EMAIL=… ADMIN_NAME="…"
	@test -n "$(ADMIN_EMAIL)" -a -n "$(ADMIN_NAME)" || { echo 'Usage: make prod-bootstrap ADMIN_EMAIL=admin@kaztbu.edu.kz ADMIN_NAME="Имя Фамилия" [BOOTSTRAP_FLAGS="-email-domains kaztbu.edu.kz"]' >&2; exit 2; }
	$(COMPOSE_PROD) run --rm --no-deps worker /app/seed bootstrap -admin-email "$(ADMIN_EMAIL)" -admin-name "$(ADMIN_NAME)" $(BOOTSTRAP_FLAGS)

prod-demo: .env ## Demo data on a presentation server (asks for the demo accounts' password)
	@read -r -s -p "Password for every demo account (at least 10 characters): " password; echo; 	  SEED_DEMO_PASSWORD="$$password" $(COMPOSE_PROD) run --rm --no-deps -e SEED_DEMO_PASSWORD worker /app/seed demo

prod-backup: .env ## Back up the database and the uploaded files into backups/
	deploy/scripts/backup.sh

prod-update: .env ## Take the new version from git and restart (backup first: make prod-backup)
	git pull --ff-only
	$(MAKE) prod-up

# ── Images ─────────────────────────────────────────────────────────────────
.PHONY: build
build: .env ## Build the api and web Docker images
	$(COMPOSE) build migrate web
