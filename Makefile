# API-Man Makefile
# Provides convenient targets for common operations

.PHONY: help generate run list envs build clean test frontend-install frontend-dev frontend-build web backend-dev dev

# Default target
help:
	@echo "API-Man Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make generate <openapi-spec>   Generate request configs from OpenAPI spec"
	@echo "  make run <request> [env]       Execute a request (defaults to dev environment)"
	@echo "  make list                      List all available requests"
	@echo "  make envs                      List all available environments"
	@echo "  make build                     Build the api-man binary"
	@echo "  make clean                     Clean build artifacts"
	@echo "  make test                      Run tests"
	@echo "  make frontend-install          Install frontend npm dependencies"
	@echo "  make frontend-dev              Run frontend Vite dev server (port 5173)"
	@echo "  make frontend-build            Build frontend static assets to frontend/dist"
	@echo "  make web [port]                Run backend web server (default port 3001)"
	@echo "  make backend-dev               Run backend with entr reloads (port 3001)"
	@echo "  make dev                       Run reloading backend + frontend dev servers together"
	@echo ""
	@echo "Examples:"
	@echo "  make generate openapi.yaml"
	@echo "  make run users/get-users"
	@echo "  make run users/get-users prod"
	@echo ""
	@echo "Custom request/body files:"
	@echo "  Place custom request.json or body.json in the same directory as existing files"
	@echo "  The tool will automatically use these files when running requests"

# Generate requests from OpenAPI spec
generate:
	@if [ -z "$(filter-out $@,$(MAKECMDGOALS))" ]; then \
		echo "Error: Please specify an OpenAPI spec file"; \
		echo "Usage: make generate <openapi-spec.yaml>"; \
		exit 1; \
	fi
	@spec_file="$(filter-out $@,$(MAKECMDGOALS))"; \
	if [ ! -f "$$spec_file" ]; then \
		echo "Error: OpenAPI spec file '$$spec_file' not found"; \
		exit 1; \
	fi; \
	echo "Generating requests from $$spec_file..."; \
	go run . generate "$$spec_file"

# Run a request with optional environment (defaults to dev)
run:
	@if [ -z "$(filter-out $@,$(MAKECMDGOALS))" ]; then \
		echo "Error: Please specify a request path"; \
		echo "Usage: make run <request-path> [environment]"; \
		echo "Example: make run users/get-users dev"; \
		exit 1; \
	fi
	@args="$(filter-out $@,$(MAKECMDGOALS))"; \
	request_path=$$(echo $$args | cut -d' ' -f1); \
	env_name=$$(echo $$args | cut -d' ' -f2); \
	if [ -z "$$env_name" ]; then \
		env_name="dev"; \
	fi; \
	echo "Running request: $$request_path (environment: $$env_name)"; \
	go run . run "$$request_path" "$$env_name"

# List all available requests
list:
	@echo "Listing all available requests..."
	@go run . list

# List all available environments
envs:
	@echo "Listing all available environments..."
	@go run . envs

# Build the binary
build:
	@echo "Building api-man binary..."
	@go build -o api-man .
	@echo "✓ Binary built as 'api-man'"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -f api-man
	@echo "✓ Clean complete"

# Run tests
test:
	@echo "Running tests..."
	@go test ./...

# Install frontend dependencies
frontend-install:
	@echo "Installing frontend dependencies..."
	@cd frontend && npm install

# Run frontend dev server (Vite, port 5173)
frontend-dev:
	@echo "Starting frontend dev server on http://localhost:5173 ..."
	@cd frontend && npm run dev

# Build frontend static assets
frontend-build:
	@echo "Building frontend..."
	@cd frontend && npm run build

# Run backend web server (defaults to port 3001, serves frontend/dist)
web: build
	@port="$(filter-out $@,$(MAKECMDGOALS))"; \
	if [ -z "$$port" ]; then port=3001; fi; \
	echo "Starting backend web server on http://localhost:$$port ..."; \
	./api-man web $$port ./frontend/dist

# Run backend web server with reloads when Go files change
backend-dev:
	@if ! command -v entr >/dev/null 2>&1; then \
		echo "Error: entr is required for backend reloads. Install it with: brew install entr"; \
		exit 1; \
	fi
	@find . \
		\( -path ./frontend -o -path ./.git \) -prune -o \
		\( -name '*.go' -o -name 'go.mod' -o -name 'go.sum' \) -print | \
		entr -nr sh -c 'go run . web 3001 ./frontend/dist'

# Run backend + frontend dev servers together
dev:
	@./dev.sh

# Prevent make from treating arguments as targets
%:
	@:
