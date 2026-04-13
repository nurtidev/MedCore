.PHONY: up down build test lint proto migrate clean logs ps help

# ─────────────────────────────────────────────────────────────────────────────
# MedCore Makefile
# ─────────────────────────────────────────────────────────────────────────────

COMPOSE     := docker compose
GO          := go
GOFLAGS     := -ldflags="-w -s"
SERVICES    := auth billing integration analytics gateway
PROTO_DIR   := pkg/proto
MIGRATE_DIR := migrations
DB_URL      ?= postgres://medcore:medcore@localhost:5432/medcore?sslmode=disable

# ── Docker Compose ────────────────────────────────────────────────────────────

up: ## Start all services (build if needed)
	$(COMPOSE) up --build -d

up-infra: ## Start infrastructure only (postgres, redis, kafka, clickhouse, gotenberg)
	$(COMPOSE) up -d postgres redis zookeeper kafka clickhouse gotenberg

down: ## Stop all services
	$(COMPOSE) down

down-v: ## Stop all services and remove volumes
	$(COMPOSE) down -v

restart: ## Restart all services
	$(COMPOSE) restart

ps: ## Show running containers
	$(COMPOSE) ps

logs: ## Tail all service logs
	$(COMPOSE) logs -f

logs-%: ## Tail logs for a specific service (e.g. make logs-gateway)
	$(COMPOSE) logs -f $*

# ── Build ────────────────────────────────────────────────────────────────────

build: ## Build all Go binaries
	$(GO) build $(GOFLAGS) ./cmd/...

build-%: ## Build a specific service binary (e.g. make build-gateway)
	$(GO) build $(GOFLAGS) -o bin/$* ./cmd/$*

# ── Test ─────────────────────────────────────────────────────────────────────

test: ## Run all tests
	$(GO) test ./... -timeout 60s

test-v: ## Run all tests with verbose output
	$(GO) test ./... -timeout 60s -v

test-%: ## Run tests for a specific package (e.g. make test-gateway)
	$(GO) test ./internal/$*/... -timeout 60s -v

cover: ## Run tests with coverage report
	$(GO) test ./... -coverprofile=coverage.out -timeout 60s
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# ── Code quality ─────────────────────────────────────────────────────────────

lint: ## Run golangci-lint
	golangci-lint run ./...

fmt: ## Format all Go files
	$(GO) fmt ./...

vet: ## Run go vet
	$(GO) vet ./...

# ── Migrations ───────────────────────────────────────────────────────────────

migrate-up: ## Apply all pending migrations
	goose -dir $(MIGRATE_DIR) postgres "$(DB_URL)" up

migrate-down: ## Roll back the last migration
	goose -dir $(MIGRATE_DIR) postgres "$(DB_URL)" down

migrate-status: ## Show migration status
	goose -dir $(MIGRATE_DIR) postgres "$(DB_URL)" status

migrate-reset: ## Roll back all migrations
	goose -dir $(MIGRATE_DIR) postgres "$(DB_URL)" reset

# ── Proto ────────────────────────────────────────────────────────────────────

proto: ## Regenerate protobuf files
	protoc \
		--go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/auth/auth.proto
	@echo "Proto files regenerated"

# ── Dev helpers ──────────────────────────────────────────────────────────────

deps: ## Download Go dependencies
	$(GO) mod download

tidy: ## Tidy Go modules
	$(GO) mod tidy

seed: ## Seed the database with test data
	$(GO) run ./cmd/seed/main.go

clean: ## Remove build artifacts
	rm -rf bin/ coverage.out coverage.html

env: ## Copy .env.example to .env (first-time setup)
	@test -f .env || (cp .env.example .env && echo "Created .env from .env.example")

# ── Help ─────────────────────────────────────────────────────────────────────

help: ## Show this help
	@grep -E '^[a-zA-Z_%-]+:.*##' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*##"}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' | sort

.DEFAULT_GOAL := help
