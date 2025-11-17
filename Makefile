.PHONY: build test clean docker-build docker-up docker-down run-example help

# Build configuration
BINARY_NAME=gress
BUILD_DIR=bin
GO=go
GOFLAGS=-v

# Docker configuration
DOCKER_COMPOSE=docker-compose
DOCKER_DIR=deployments/docker

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the application
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/gress

test: ## Run tests
	@echo "Running tests..."
	$(GO) test ./... -v -cover

test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	$(GO) test ./... -coverprofile=coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

bench: ## Run benchmarks
	@echo "Running benchmarks..."
	$(GO) test ./... -bench=. -benchmem

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	cd $(DOCKER_DIR) && docker build -t gress:latest -f Dockerfile ../..

docker-up: ## Start Docker Compose stack
	@echo "Starting Docker Compose stack..."
	cd $(DOCKER_DIR) && $(DOCKER_COMPOSE) up -d

docker-down: ## Stop Docker Compose stack
	@echo "Stopping Docker Compose stack..."
	cd $(DOCKER_DIR) && $(DOCKER_COMPOSE) down

docker-logs: ## View Docker Compose logs
	cd $(DOCKER_DIR) && $(DOCKER_COMPOSE) logs -f

docker-clean: ## Clean Docker volumes
	@echo "Cleaning Docker volumes..."
	cd $(DOCKER_DIR) && $(DOCKER_COMPOSE) down -v

run: build ## Build and run the application
	@echo "Running $(BINARY_NAME)..."
	./$(BUILD_DIR)/$(BINARY_NAME)

run-example: ## Run the ride-sharing example
	@echo "Running ride-sharing example..."
	cd examples/rideshare && $(GO) run main.go simulator.go

fmt: ## Format code
	@echo "Formatting code..."
	$(GO) fmt ./...

lint: ## Run linter
	@echo "Running linter..."
	golangci-lint run

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GO) mod download

deps-update: ## Update dependencies
	@echo "Updating dependencies..."
	$(GO) get -u ./...
	$(GO) mod tidy

all: clean deps build test ## Clean, download deps, build, and test

.DEFAULT_GOAL := help
