# VoidRunner Makefile
# Provides standardized commands for building, testing, and running the application

.PHONY: help test test-fast test-integration test-all build run dev clean coverage coverage-check docs docs-serve lint fmt vet security deps deps-update migrate-up migrate-down migrate-reset migration docker-build docker-run clean-docs install-tools setup all pre-commit bench db-start db-stop db-reset db-status dev-up dev-down dev-logs dev-restart dev-status prod-up prod-down prod-logs prod-restart prod-status docker-clean env-status

# Default target
help: ## Show this help message
	@echo "VoidRunner API Development Commands"
	@echo "=================================="
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

# Build targets
build: ## Build the API server binary
	@echo "Building VoidRunner API server..."
	@go build -o bin/voidrunner-api ./cmd/api
	@echo "Build complete: bin/voidrunner-api"


# Test targets
test: ## Run unit tests only (with coverage if CI=true)
	@echo "Running unit tests (excluding integration tests)..."
ifeq ($(CI),true)
	@go test -race -coverprofile=coverage.out ./cmd/... ./internal/... ./pkg/...
	@go tool cover -func=coverage.out
	@./scripts/check-coverage.sh coverage.out 80.0 || echo "Coverage check failed but not blocking CI"
else
	@go test -cover -race ./cmd/... ./internal/... ./pkg/...
endif

test-fast: ## Run unit tests in short mode (fast, excluding integration)
	@echo "Running unit tests (fast mode, excluding integration tests)..."
	@go test -short ./cmd/... ./internal/... ./pkg/...


test-integration: ## Run integration tests (requires database to be running)
	@echo "Running integration tests..."
	@echo "Note: Database must be running before running integration tests"
	@echo "Use 'make db-start' to start the test database"
	@INTEGRATION_TESTS=true \
	 TEST_DB_HOST=$${TEST_DB_HOST:-localhost} \
	 TEST_DB_PORT=$${TEST_DB_PORT:-5432} \
	 TEST_DB_USER=$${TEST_DB_USER:-testuser} \
	 TEST_DB_PASSWORD=$${TEST_DB_PASSWORD:-testpassword} \
	 TEST_DB_NAME=$${TEST_DB_NAME:-voidrunner_test} \
	 TEST_DB_SSLMODE=$${TEST_DB_SSLMODE:-disable} \
	 JWT_SECRET_KEY=$${JWT_SECRET_KEY:-test-secret-key-for-integration} \
	 go test -tags=integration -v -race ./tests/integration/...

test-all: test test-integration ## Run both unit and integration tests (database must be running)

coverage: ## Run unit tests with coverage (check threshold if CI=true)
	@echo "Running unit tests with coverage (excluding integration tests)..."
	@go test -v -race -coverprofile=coverage.out ./cmd/... ./internal/... ./pkg/...
	@go tool cover -func=coverage.out
ifeq ($(CI),true)
	@./scripts/check-coverage.sh coverage.out 80.0
else
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"
endif

coverage-check: ## Check if coverage meets minimum threshold (80%)
	@echo "Checking coverage threshold (excluding integration tests)..."
	@go test -v -race -coverprofile=coverage.out ./cmd/... ./internal/... ./pkg/...
	@go tool cover -func=coverage.out
	@./scripts/check-coverage.sh coverage.out 80.0


# Development targets
run: build ## Run the API server locally
	@echo "Starting VoidRunner API server..."
	@./bin/voidrunner-api

dev: ## Run in development mode with auto-reload
	@echo "Starting development server..."
	@air || go run ./cmd/api

# Code quality targets
fmt: ## Format Go code
	@echo "Formatting Go code..."
	@go fmt ./...

vet: ## Run go vet
	@echo "Running go vet..."
	@go vet ./...

lint: ## Run all linting checks (including format check if CI=true)
	@echo "Running golangci-lint..."
	@golangci-lint run $(if $(CI),--timeout=5m)
ifeq ($(CI),true)
	@echo "Checking code formatting..."
	@if [ "$$(gofmt -s -l . | wc -l)" -gt 0 ]; then \
		echo "Code is not formatted properly:"; \
		gofmt -s -l .; \
		exit 1; \
	fi
endif

# Documentation targets
docs: ## Generate Swagger documentation
	@echo "Generating Swagger documentation..."
	@swag init -g cmd/api/main.go -o docs/
	@echo "Documentation generated in docs/"

docs-serve: docs ## Serve documentation locally
	@echo "Serving documentation at http://localhost:8081"
	@cd docs && python3 -m http.server 8081 || python -m SimpleHTTPServer 8081

# Database targets
migrate-up: ## Run database migrations up
	@echo "Running database migrations..."
	@go run ./cmd/migrate up

migrate-down: ## Run database migrations down (rollback one)
	@echo "Rolling back last migration..."
	@go run ./cmd/migrate down

migrate-reset: ## Reset database (rollback all migrations)
	@echo "Resetting database..."
	@go run ./cmd/migrate reset

migration: ## Create a new migration file (usage: make migration name=migration_name)
	@if [ -z "$(name)" ]; then echo "Usage: make migration name=migration_name"; exit 1; fi
	@echo "Creating migration: $(name)"
	@mkdir -p migrations
	@touch migrations/$(shell date +%Y%m%d%H%M%S)_$(name).up.sql
	@touch migrations/$(shell date +%Y%m%d%H%M%S)_$(name).down.sql
	@echo "Migration files created in migrations/"

# Database management targets
db-start: ## Start test database (Docker)
	@echo "Starting test database..."
	@./scripts/start-test-db.sh

db-stop: ## Stop test database (Docker)
	@echo "Stopping test database..."
	@./scripts/stop-test-db.sh

db-reset: ## Reset test database (clean slate)
	@echo "Resetting test database..."
	@./scripts/reset-test-db.sh

db-status: ## Show test database status
	@echo "Test database status:"
	@if command -v docker-compose &> /dev/null; then \
		docker-compose -f docker-compose.test.yml ps; \
	else \
		echo "docker-compose not found"; \
	fi

# Environment Management
dev-up: ## Start development environment (DB + Redis + API with embedded workers)
	@echo "Starting development environment..."
	@./scripts/dev-env.sh up

dev-down: ## Stop development environment
	@echo "Stopping development environment..."
	@./scripts/dev-env.sh down

dev-logs: ## Show development environment logs
	@echo "Showing development logs..."
	@./scripts/dev-env.sh logs

dev-restart: ## Restart development environment
	@echo "Restarting development environment..."
	@./scripts/dev-env.sh restart

dev-status: ## Show development environment status
	@echo "Development environment status:"
	@./scripts/dev-env.sh status

prod-up: ## Start production environment
	@echo "Starting production environment..."
	@./scripts/prod-env.sh up

prod-down: ## Stop production environment
	@echo "Stopping production environment..."
	@./scripts/prod-env.sh down

prod-logs: ## Show production environment logs
	@echo "Showing production logs..."
	@./scripts/prod-env.sh logs

prod-restart: ## Restart production environment
	@echo "Restarting production environment..."
	@./scripts/prod-env.sh restart

prod-status: ## Show production environment status
	@echo "Production environment status:"
	@./scripts/prod-env.sh status

env-status: ## Show all environment status
	@echo "=== Development Environment ==="
	@./scripts/dev-env.sh status || echo "Development environment not running"
	@echo ""
	@echo "=== Production Environment ==="
	@./scripts/prod-env.sh status || echo "Production environment not running"
	@echo ""
	@echo "=== Test Database ==="
	@if command -v docker-compose &> /dev/null; then \
		if docker-compose -f docker-compose.test.yml ps | grep -q "Up"; then \
			echo "Test database: Running"; \
		else \
			echo "Test database: Stopped"; \
		fi; \
	else \
		echo "docker-compose not found"; \
	fi

docker-clean: ## Clean Docker resources (containers, images, volumes)
	@echo "Cleaning Docker resources..."
	@echo "WARNING: This will remove all VoidRunner containers, images, and volumes"
	@read -p "Are you sure? [y/N] " -n 1 -r && echo && \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		docker-compose -f docker-compose.yml down -v --remove-orphans 2>/dev/null || true; \
		docker-compose -f docker-compose.dev.yml down -v --remove-orphans 2>/dev/null || true; \
		docker-compose -f docker-compose.test.yml down -v --remove-orphans 2>/dev/null || true; \
		docker system prune -f --volumes; \
		echo "Docker cleanup complete"; \
	else \
		echo "Docker cleanup cancelled"; \
	fi


# Dependency management
deps: ## Download and tidy dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

deps-update: ## Update dependencies
	@echo "Updating dependencies..."
	@go get -u ./...
	@go mod tidy

# Security targets
security: ## Run security scan (SARIF format if CI=true)
	@echo "Running security scan..."
ifeq ($(CI),true)
	@gosec -no-fail -fmt sarif -out gosec.sarif ./...
else
	@gosec ./... || echo "Security scan completed with findings"
endif

# Docker targets
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -t voidrunner-api .

docker-run: docker-build ## Run in Docker container
	@echo "Running in Docker container..."
	@docker run -p 8080:8080 --env-file .env voidrunner-api

# Clean targets
clean: ## Clean build artifacts and caches
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@rm -f coverage.out coverage.html
	@go clean -cache -testcache -modcache
	@echo "Clean complete"

clean-docs: ## Clean generated documentation
	@echo "Cleaning generated documentation..."
	@rm -f docs/docs.go docs/swagger.json docs/swagger.yaml

# Install development tools
install-tools: ## Install development tools
	@echo "Installing development tools..."
	@go install github.com/swaggo/swag/cmd/swag@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/securego/gosec/v2/cmd/gosec@latest
	@go install github.com/air-verse/air@latest
	@echo "Development tools installed"


# Environment setup
setup: deps install-tools ## Setup development environment
	@echo "Setting up development environment..."
	@cp .env.example .env || echo ".env.example not found, skipping"
	@echo "✅ Development environment setup complete!"
	@echo "  - Tools: installed"
	@echo "  - Dependencies: downloaded"
	@echo "  - .env file: created (edit with your configuration)"
	@echo ""
	@echo "To run integration tests:"
	@echo "  make db-start         # Start test database"
	@echo "  make test-integration # Run integration tests"
	@echo "  make db-stop          # Stop test database (optional)"

# Performance testing
bench: ## Run benchmark tests
	@echo "Running benchmark tests..."
	@go test -bench=. -benchmem ./...


all: clean deps lint test build ## Run all quality checks and build

# Precommit hook
pre-commit: fmt vet lint test ## Run pre-commit checks
	@echo "Pre-commit checks passed"
