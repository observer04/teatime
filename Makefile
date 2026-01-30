# ==============================================================================
# TeaTime Makefile
# ==============================================================================
# One-command development workflow
# ==============================================================================

.PHONY: help dev dev-down build test lint migrate migrate-down clean logs

# Default target
help:
	@echo "TeaTime Development Commands"
	@echo "============================"
	@echo "  make dev          - Start all services (postgres + backend + frontend)"
	@echo "  make dev-down     - Stop all services"
	@echo "  make build        - Build backend Docker image"
	@echo "  make test         - Run all tests"
	@echo "  make lint         - Run linters"
	@echo "  make migrate      - Run database migrations"
	@echo "  make migrate-down - Rollback last migration"
	@echo "  make logs         - Tail all logs"
	@echo "  make logs-backend - Tail backend logs"
	@echo "  make logs-frontend- Tail frontend logs"
	@echo "  make clean        - Remove containers, volumes, build artifacts"
	@echo ""

# ==============================================================================
# Development
# ==============================================================================

# Start development environment (Postgres + backend + frontend)
dev:
	docker compose up -d

# Alternative: Start with backend hot-reload
dev-hot:
	docker compose up -d postgres
	@echo "Waiting for Postgres to be ready..."
	@sleep 2
	docker compose --profile dev up backend-dev frontend

# Alternative: Start without hot-reload (uses compiled binary)
dev-prod:
	docker compose up --build

# Stop all services
dev-down:
	docker compose down

# View logs
logs:
	docker compose logs -f

logs-backend:
	docker compose logs -f backend

logs-frontend:
	docker compose logs -f frontend

logs-db:
	docker compose logs -f postgres

# ==============================================================================
# Build
# ==============================================================================

build:
	docker compose build backend

build-local:
	cd backend && go build -o bin/server ./cmd/server

# ==============================================================================
# Testing
# ==============================================================================

test:
	cd backend && go test -v -race -cover ./...

test-short:
	cd backend && go test -v -short ./...

# Integration tests (requires running Postgres)
test-integration:
	cd backend && go test -v -tags=integration ./...

# ==============================================================================
# Linting
# ==============================================================================

lint:
	cd backend && go vet ./...
	cd backend && test -z "$$(gofmt -l .)" || (gofmt -d . && exit 1)

lint-fix:
	cd backend && gofmt -w .

# With golangci-lint (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
lint-full:
	cd backend && golangci-lint run

# ==============================================================================
# Database
# ==============================================================================

# Run migrations (requires migrate CLI or use Docker)
migrate:
	docker compose exec postgres psql -U teatime -d teatime -f /dev/stdin < backend/migrations/*.up.sql 2>/dev/null || echo "No migrations yet or use golang-migrate"

# Connect to database
db-shell:
	docker compose exec postgres psql -U teatime -d teatime

# ==============================================================================
# Cleanup
# ==============================================================================

clean:
	docker compose down -v --remove-orphans
	rm -rf backend/bin backend/tmp
	@echo "Cleaned up containers, volumes, and build artifacts"

# ==============================================================================
# Quick health check
# ==============================================================================

health:
	@curl -s http://localhost:8080/healthz | jq . || curl -s http://localhost:8080/healthz
