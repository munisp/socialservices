# Social Protection Platform - Makefile
# Common operations for development, testing, and deployment

.PHONY: help setup dev build test lint clean docker-up docker-down migrate seed

# Default target
help:
	@echo "Social Protection Platform - Available Commands"
	@echo ""
	@echo "Setup & Development:"
	@echo "  make setup          - Install all dependencies"
	@echo "  make dev            - Start development servers"
	@echo "  make build          - Build all services"
	@echo ""
	@echo "Testing:"
	@echo "  make test           - Run all tests"
	@echo "  make test-unit      - Run unit tests only"
	@echo "  make test-e2e       - Run end-to-end tests"
	@echo "  make lint           - Run linters"
	@echo "  make typecheck      - Run type checking"
	@echo ""
	@echo "Database:"
	@echo "  make migrate        - Run database migrations"
	@echo "  make seed           - Seed database with test data"
	@echo "  make db-reset       - Reset database (drop, create, migrate, seed)"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-up      - Start all services with Docker Compose"
	@echo "  make docker-down    - Stop all Docker services"
	@echo "  make docker-logs    - View Docker logs"
	@echo ""
	@echo "Go Services:"
	@echo "  make go-build       - Build Go orchestrator"
	@echo "  make go-test        - Run Go tests"
	@echo "  make go-lint        - Run Go linter"
	@echo ""
	@echo "Python Services:"
	@echo "  make py-install     - Install Python dependencies"
	@echo "  make py-test        - Run Python tests"
	@echo "  make py-lint        - Run Python linter"
	@echo ""
	@echo "Deployment:"
	@echo "  make deploy-staging - Deploy to staging"
	@echo "  make deploy-prod    - Deploy to production (requires approval)"

# ============================================
# Setup & Development
# ============================================

setup: setup-node setup-go setup-python
	@echo "Setup complete!"

setup-node:
	@echo "Installing Node.js dependencies..."
	npm install

setup-go:
	@echo "Installing Go dependencies..."
	cd orchestrator && go mod download

setup-python:
	@echo "Installing Python dependencies..."
	cd python-services/pmt-service && pip install -r requirements.txt
	cd services/fraud-ml && pip install -r requirements.txt

dev:
	@echo "Starting development servers..."
	npm run dev

build: build-client build-server build-go
	@echo "Build complete!"

build-client:
	@echo "Building client..."
	npm run build:client

build-server:
	@echo "Building server..."
	npm run build:server

build-go:
	@echo "Building Go orchestrator..."
	cd orchestrator && go build -o bin/orchestrator ./cmd/main.go

# ============================================
# Testing
# ============================================

test: test-unit test-e2e
	@echo "All tests complete!"

test-unit:
	@echo "Running unit tests..."
	npm run test:unit

test-e2e:
	@echo "Running e2e tests..."
	npm run test:e2e

lint: lint-ts lint-go lint-py
	@echo "Linting complete!"

lint-ts:
	@echo "Linting TypeScript..."
	npm run lint

lint-go:
	@echo "Linting Go..."
	cd orchestrator && golangci-lint run ./...

lint-py:
	@echo "Linting Python..."
	cd python-services && ruff check .

typecheck:
	@echo "Running type checks..."
	npm run typecheck

# ============================================
# Database
# ============================================

migrate:
	@echo "Running database migrations..."
	npm run db:migrate

seed:
	@echo "Seeding database..."
	npm run db:seed

db-reset:
	@echo "Resetting database..."
	npm run db:reset

# ============================================
# Docker
# ============================================

docker-up:
	@echo "Starting Docker services..."
	docker compose -f docker-compose.dev.yml up -d

docker-down:
	@echo "Stopping Docker services..."
	docker compose -f docker-compose.dev.yml down

docker-logs:
	@echo "Viewing Docker logs..."
	docker compose -f docker-compose.dev.yml logs -f

docker-build:
	@echo "Building Docker images..."
	docker compose -f docker-compose.dev.yml build

# ============================================
# Go Services
# ============================================

go-build:
	@echo "Building Go services..."
	cd orchestrator && go build -o bin/orchestrator ./cmd/main.go

go-test:
	@echo "Running Go tests..."
	cd orchestrator && go test -v ./...

go-lint:
	@echo "Running Go linter..."
	cd orchestrator && golangci-lint run ./...

go-run:
	@echo "Running Go orchestrator..."
	cd orchestrator && go run ./cmd/main.go

# ============================================
# Python Services
# ============================================

py-install:
	@echo "Installing Python dependencies..."
	pip install -r python-services/pmt-service/requirements.txt
	pip install -r services/fraud-ml/requirements.txt

py-test:
	@echo "Running Python tests..."
	cd python-services && pytest

py-lint:
	@echo "Running Python linter..."
	cd python-services && ruff check .

py-format:
	@echo "Formatting Python code..."
	cd python-services && ruff format .

# ============================================
# Deployment
# ============================================

deploy-staging:
	@echo "Deploying to staging..."
	kubectl apply -k k8s/overlays/staging

deploy-prod:
	@echo "Deploying to production..."
	@echo "WARNING: This will deploy to production!"
	@read -p "Are you sure? [y/N] " confirm && [ "$$confirm" = "y" ]
	kubectl apply -k k8s/overlays/production

# ============================================
# Utilities
# ============================================

clean:
	@echo "Cleaning build artifacts..."
	rm -rf dist node_modules/.cache
	cd orchestrator && rm -rf bin

logs:
	@echo "Viewing application logs..."
	npm run logs

health-check:
	@echo "Checking service health..."
	curl -s http://localhost:3000/health | jq
	curl -s http://localhost:8090/health | jq

metrics:
	@echo "Viewing metrics..."
	curl -s http://localhost:8090/metrics
