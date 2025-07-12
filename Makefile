.PHONY: help build test lint fmt clean install-tools deps run dev coverage bench

# Default target
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Build targets
build: ## Build the application
	@echo "Building tf-analyzer..."
	@go build -ldflags="-s -w" -o bin/tf-analyzer ./cmd/tf-analyzer

build-race: ## Build with race detector
	@echo "Building with race detector..."
	@go build -race -o bin/tf-analyzer-race ./cmd/tf-analyzer

# Development targets
run: ## Run the application
	@echo "Running tf-analyzer..."
	@go run ./cmd/tf-analyzer

dev: ## Run with live reload (requires air)
	@echo "Starting development server..."
	@air

# Testing targets
test: ## Run tests
	@echo "Running tests..."
	@go test -v ./...

test-race: ## Run tests with race detector
	@echo "Running tests with race detector..."
	@go test -race -v ./...

test-short: ## Run short tests only
	@echo "Running short tests..."
	@go test -short -v ./...

coverage: ## Generate test coverage report
	@echo "Generating coverage report..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

bench: ## Run benchmarks
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./...

# Code quality targets
lint: install-golangci-lint ## Run golangci-lint
	@echo "Running golangci-lint..."
	@golangci-lint run

lint-fix: install-golangci-lint ## Run golangci-lint with auto-fix
	@echo "Running golangci-lint with auto-fix..."
	@golangci-lint run --fix

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...

# Tool installation
install-tools: install-golangci-lint install-air ## Install all development tools

install-golangci-lint: ## Install golangci-lint
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v1.55.2)

install-air: ## Install air for live reload
	@which air > /dev/null || (echo "Installing air..." && \
		go install github.com/air-verse/air@latest)

# Dependency management
deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download

deps-tidy: ## Tidy up dependencies
	@echo "Tidying dependencies..."
	@go mod tidy

deps-verify: ## Verify dependencies
	@echo "Verifying dependencies..."
	@go mod verify

# Cleanup
clean: ## Clean build artifacts
	@echo "Cleaning up..."
	@rm -rf bin/ dist/ tmp/ coverage.out coverage.html

# CI targets
ci: deps lint test ## Run CI pipeline (lint + test)

ci-full: deps lint test-race coverage ## Run full CI pipeline

# Security
sec: ## Run security checks (requires gosec)
	@which gosec > /dev/null || go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	@echo "Running security checks..."
	@gosec ./...

# Initialize project
init: ## Initialize go module and install tools
	@echo "Initializing project..."
	@[ -f go.mod ] || go mod init tf-analyzer
	@make install-tools
	@make deps

# Performance profiling
profile-cpu: ## Run CPU profiling
	@echo "Running CPU profiling..."
	@go test -cpuprofile=cpu.prof -bench=. ./...

profile-mem: ## Run memory profiling
	@echo "Running memory profiling..."
	@go test -memprofile=mem.prof -bench=. ./...