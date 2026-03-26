# Makefile for ChatApp

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Binary names
BINARY_DIR=bin
GATEWAY_BINARY=$(BINARY_DIR)/gateway
PROCESSOR_BINARY=$(BINARY_DIR)/processor
PRESENCE_BINARY=$(BINARY_DIR)/presence
FANOUT_BINARY=$(BINARY_DIR)/fanout
API_BINARY=$(BINARY_DIR)/api
BENCHMARK_BINARY=$(BINARY_DIR)/benchmark

# Docker variables
DOCKER_REGISTRY=chatapp
VERSION?=latest
IMAGE_TAG=$(DOCKER_REGISTRY)/chatapp:$(VERSION)

# Kubernetes variables
NAMESPACE?=chatapp
ENVIRONMENT?=development

# Test variables
TEST_TIMEOUT=30s
TEST_COVERAGE=coverage.out
TEST_COVERAGE_HTML=coverage.html

# Linting variables
LINT_CONFIG=.golangci.yml

.PHONY: help
help: ## Display this help screen
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 3) } ' $(MAKEFILE_LIST)

##@ Build

.PHONY: build
build: ## Build all binaries
	@echo "Building all binaries..."
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) -o $(GATEWAY_BINARY) ./cmd/gateway
	$(GOBUILD) -o $(PROCESSOR_BINARY) ./cmd/processor
	$(GOBUILD) -o $(PRESENCE_BINARY) ./cmd/presence
	$(GOBUILD) -o $(FANOUT_BINARY) ./cmd/fanout
	$(GOBUILD) -o $(API_BINARY) ./cmd/api
	$(GOBUILD) -o $(BENCHMARK_BINARY) ./benchmark

.PHONY: build-gateway
build-gateway: ## Build gateway binary
	@echo "Building gateway..."
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) -o $(GATEWAY_BINARY) ./cmd/gateway

.PHONY: build-processor
build-processor: ## Build processor binary
	@echo "Building processor..."
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) -o $(PROCESSOR_BINARY) ./cmd/processor

.PHONY: build-presence
build-presence: ## Build presence binary
	@echo "Building presence..."
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) -o $(PRESENCE_BINARY) ./cmd/presence

.PHONY: build-fanout
build-fanout: ## Build fanout binary
	@echo "Building fanout..."
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) -o $(FANOUT_BINARY) ./cmd/fanout

.PHONY: build-api
build-api: ## Build API binary
	@echo "Building API..."
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) -o $(API_BINARY) ./cmd/api

.PHONY: build-benchmark
build-benchmark: ## Build benchmark binary
	@echo "Building benchmark..."
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) -o $(BENCHMARK_BINARY) ./benchmark

##@ Test

.PHONY: test
test: ## Run all tests
	@echo "Running tests..."
	$(GOTEST) -v -timeout $(TEST_TIMEOUT) ./...

.PHONY: test-short
test-short: ## Run short tests
	@echo "Running short tests..."
	$(GOTEST) -v -short -timeout $(TEST_TIMEOUT) ./...

.PHONY: test-integration
test-integration: ## Run integration tests
	@echo "Running integration tests..."
	$(GOTEST) -v -tags=integration -timeout $(TEST_TIMEOUT) ./...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	$(GOTEST) -v -timeout $(TEST_TIMEOUT) -coverprofile=$(TEST_COVERAGE) ./...
	$(GOCMD) tool cover -html=$(TEST_COVERAGE) -o $(TEST_COVERAGE_HTML)
	@echo "Coverage report generated: $(TEST_COVERAGE_HTML)"

.PHONY: test-race
test-race: ## Run tests with race detector
	@echo "Running tests with race detector..."
	$(GOTEST) -v -race -timeout $(TEST_TIMEOUT) ./...

.PHONY: benchmark
benchmark: ## Run benchmarks
	@echo "Running benchmarks..."
	$(GOTEST) -v -bench=. -benchmem ./...

##@ Quality

.PHONY: fmt
fmt: ## Format code
	@echo "Formatting code..."
	$(GOFMT) ./...

.PHONY: vet
vet: ## Run go vet
	@echo "Running go vet..."
	$(GOVET) ./...

.PHONY: lint
lint: ## Run linter
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --config $(LINT_CONFIG); \
	else \
		echo "golangci-lint not installed, skipping..."; \
	fi

.PHONY: security
security: ## Run security scan
	@echo "Running security scan..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "gosec not installed, skipping..."; \
	fi

.PHONY: check
check: fmt vet lint security test ## Run all quality checks

##@ Dependencies

.PHONY: deps
deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) verify

.PHONY: deps-update
deps-update: ## Update dependencies
	@echo "Updating dependencies..."
	$(GOGET) -u ./...
	$(GOMOD) tidy

.PHONY: deps-clean
deps-clean: ## Clean dependencies
	@echo "Cleaning dependencies..."
	$(GOMOD) clean

##@ Docker

.PHONY: docker-build
docker-build: ## Build Docker images
	@echo "Building Docker images..."
	docker build -f build/gateway/Dockerfile -t $(DOCKER_REGISTRY)/gateway:$(VERSION) .
	docker build -f build/processor/Dockerfile -t $(DOCKER_REGISTRY)/processor:$(VERSION) .
	docker build -f build/presence/Dockerfile -t $(DOCKER_REGISTRY)/presence:$(VERSION) .
	docker build -f build/fanout/Dockerfile -t $(DOCKER_REGISTRY)/fanout:$(VERSION) .
	docker build -f build/api/Dockerfile -t $(DOCKER_REGISTRY)/api:$(VERSION) .

.PHONY: docker-push
docker-push: ## Push Docker images
	@echo "Pushing Docker images..."
	docker push $(DOCKER_REGISTRY)/gateway:$(VERSION)
	docker push $(DOCKER_REGISTRY)/processor:$(VERSION)
	docker push $(DOCKER_REGISTRY)/presence:$(VERSION)
	docker push $(DOCKER_REGISTRY)/fanout:$(VERSION)
	docker push $(DOCKER_REGISTRY)/api:$(VERSION)

.PHONY: docker-pull
docker-pull: ## Pull Docker images
	@echo "Pulling Docker images..."
	docker pull $(DOCKER_REGISTRY)/gateway:$(VERSION)
	docker pull $(DOCKER_REGISTRY)/processor:$(VERSION)
	docker pull $(DOCKER_REGISTRY)/presence:$(VERSION)
	docker pull $(DOCKER_REGISTRY)/fanout:$(VERSION)
	docker pull $(DOCKER_REGISTRY)/api:$(VERSION)

##@ Kubernetes

.PHONY: k8s-deploy
k8s-deploy: ## Deploy to Kubernetes
	@echo "Deploying to Kubernetes..."
	./scripts/deploy.sh $(ENVIRONMENT) all

.PHONY: k8s-undeploy
k8s-undeploy: ## Undeploy from Kubernetes
	@echo "Undeploying from Kubernetes..."
	helm uninstall chatapp -n $(NAMESPACE) || true

.PHONY: k8s-status
k8s-status: ## Check Kubernetes status
	@echo "Checking Kubernetes status..."
	kubectl get pods -n $(NAMESPACE)
	kubectl get services -n $(NAMESPACE)

.PHONY: k8s-logs
k8s-logs: ## Show Kubernetes logs
	@echo "Showing Kubernetes logs..."
	kubectl logs -f -n $(NAMESPACE) -l app.kubernetes.io/name=chatapp

##@ Development

.PHONY: dev-setup
dev-setup: ## Set up development environment
	@echo "Setting up development environment..."
	$(GOMOD) download
	@if command -v golangci-lint >/dev/null 2>&1; then \
		echo "golangci-lint already installed"; \
	else \
		echo "Installing golangci-lint..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.54.2; \
	fi
	@if command -v gosec >/dev/null 2>&1; then \
		echo "gosec already installed"; \
	else \
		echo "Installing gosec..."; \
		$(GOGET) github.com/securecodewarrior/gosec/v2/cmd/gosec@latest; \
	fi

.PHONY: dev-run
dev-run: ## Run all services locally
	@echo "Running all services locally..."
	./scripts/dev-run.sh

.PHONY: dev-stop
dev-stop: ## Stop all services
	@echo "Stopping all services..."
	./scripts/dev-stop.sh

##@ Local Infrastructure

.PHONY: infra-up
infra-up: ## Start local infrastructure
	@echo "Starting local infrastructure..."
	docker-compose up -d

.PHONY: infra-down
infra-down: ## Stop local infrastructure
	@echo "Stopping local infrastructure..."
	docker-compose down

.PHONY: infra-logs
infra-logs: ## Show infrastructure logs
	@echo "Showing infrastructure logs..."
	docker-compose logs -f

.PHONY: infra-reset
infra-reset: ## Reset local infrastructure
	@echo "Resetting local infrastructure..."
	docker-compose down -v
	docker-compose up -d

##@ Benchmarking

.PHONY: benchmark-run
benchmark-run: ## Run load tests
	@echo "Running load tests..."
	./scripts/run_benchmarks.sh

.PHONY: benchmark-stress
benchmark-stress: ## Run stress tests
	@echo "Running stress tests..."
	./scripts/run_benchmarks.sh stress

.PHONY: benchmark-report
benchmark-report: ## Generate benchmark report
	@echo "Generating benchmark report..."
	./scripts/generate_benchmark_report.sh

##@ CI/CD

.PHONY: ci
ci: ## Run CI pipeline
	@echo "Running CI pipeline..."
	./scripts/ci_cd_pipeline.sh $(ENVIRONMENT) main $(VERSION)

.PHONY: ci-test
ci-test: ## Run CI tests only
	@echo "Running CI tests..."
	./scripts/ci_cd_pipeline.sh $(ENVIRONMENT) main $(VERSION) test-only

.PHONY: ci-deploy
ci-deploy: ## Run CI deployment only
	@echo "Running CI deployment..."
	./scripts/ci_cd_pipeline.sh $(ENVIRONMENT) main $(VERSION) deploy-only

##@ Clean

.PHONY: clean
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	$(GOCLEAN)
	rm -rf $(BINARY_DIR)
	rm -f $(TEST_COVERAGE) $(TEST_COVERAGE_HTML)

.PHONY: clean-all
clean-all: clean ## Clean everything including Docker
	@echo "Cleaning everything..."
	docker system prune -f
	docker volume prune -f

##@ Utilities

.PHONY: version
version: ## Show version information
	@echo "ChatApp version: $(VERSION)"
	@echo "Go version: $(shell go version)"
	@echo "Git commit: $(shell git rev-parse HEAD)"
	@echo "Git branch: $(shell git rev-parse --abbrev-ref HEAD)"

.PHONY: list
list: ## List all targets
	@$(MAKE) -pRrq -f $(firstword $(MAKEFILE_LIST)) : 2>/dev/null | awk -v RS= -F: '/^# File/,/^# Finished Make data base/ {if ($$1 !~ "^[#.]") {print $$1}}' | sort | grep -E -v -e '^[^[:alnum:]]' -e '^$@$$'

.PHONY: help-advanced
help-advanced: ## Display advanced help
	@echo "Advanced targets:"
	@echo "  build-all           - Build all targets"
	@echo "  test-all            - Run all tests"
	@echo "  quality-all         - Run all quality checks"
	@echo "  deploy-all          - Deploy all environments"
	@echo ""
	@echo "Environment variables:"
	@echo "  VERSION             - Build version (default: latest)"
	@echo "  NAMESPACE           - Kubernetes namespace (default: chatapp)"
	@echo "  ENVIRONMENT         - Deployment environment (default: development)"
	@echo "  TEST_TIMEOUT        - Test timeout (default: 30s)"

.PHONY: build-all
build-all: build docker-build ## Build all binaries and Docker images

.PHONY: test-all
test-all: test test-integration test-coverage benchmark ## Run all tests

.PHONY: quality-all
quality-all: check security ## Run all quality checks

.PHONY: deploy-all
deploy-all: k8s-deploy ## Deploy all services

# Default target
.DEFAULT_GOAL := help
