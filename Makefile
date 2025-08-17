# API-Man Makefile
# Provides convenient targets for common operations

.PHONY: help generate run list envs build clean test

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

# Prevent make from treating arguments as targets
%:
	@: