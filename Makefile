# ChaosLabs Development and CI/CD Makefile
# This Makefile provides convenient commands for development, testing, and deployment

.DEFAULT_GOAL := help
.PHONY: help dev build test lint clean docker-dev docker-build setup

# Colors for output
BLUE := \033[36m
GREEN := \033[32m
YELLOW := \033[33m
RED := \033[31m
NC := \033[0m

# Project configuration
PROJECT_NAME := chaoslabs
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")

# Go configuration
GO_VERSION := 1.21
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

# Build flags
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)
BUILD_FLAGS := -ldflags="$(LDFLAGS)" -trimpath

help: ## Show this help message
	@echo "$(BLUE)ChaosLabs Development Commands$(NC)"
	@echo "=============================="
	@echo ""
	@echo "$(GREEN)Development:$(NC)"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / && /Development/ {found=1; next} found && /^[a-zA-Z_-]+:.*?## / && !/Development/ {found=0} found {printf "  $(BLUE)%-20s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ""
	@echo "$(GREEN)Building & Testing:$(NC)"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / && (/Build/ || /Test/) {printf "  $(BLUE)%-20s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ""
	@echo "$(GREEN)Docker & Deployment:$(NC)"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / && (/Docker/ || /Deploy/) {printf "  $(BLUE)%-20s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ""
	@echo "$(GREEN)Quality & Analysis:$(NC)"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / && (/Quality/ || /Analysis/) {printf "  $(BLUE)%-20s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST)

## Development Commands

setup: ## Development - Set up development environment
	@echo "$(BLUE)Setting up development environment...$(NC)"
ifeq ($(OS),Windows_NT)
	@powershell -ExecutionPolicy Bypass -File infrastructure/devtools/scripts/dev-setup.ps1
else
	@chmod +x infrastructure/devtools/scripts/dev-setup.sh
	@infrastructure/devtools/scripts/dev-setup.sh
endif

dev: ## Development - Start complete development environment
	@echo "$(BLUE)Starting development environment...$(NC)"
	@docker-compose -f infrastructure/docker-compose.dev.yml up --build

dev-controller: ## Development - Start controller with hot reload
	@echo "$(BLUE)Starting controller with hot reload...$(NC)"
	@air -c .air.toml

dev-agent: ## Development - Start agent in development mode
	@echo "$(BLUE)Starting agent in development mode...$(NC)"
	@cd agent && go run -ldflags="$(LDFLAGS)" .

dev-frontend: ## Development - Start frontend development server
	@echo "$(BLUE)Starting frontend development server...$(NC)"
	@cd dashboard-v2 && npm run dev

dev-cli: ## Development - Build and test CLI in development mode
	@echo "$(BLUE)Building CLI in development mode...$(NC)"
	@cd cli && go run -ldflags="$(LDFLAGS)" . --help

dev-tools: ## Development - Start development tools container
	@echo "$(BLUE)Starting development tools container...$(NC)"
	@docker-compose -f infrastructure/docker-compose.dev.yml run --rm devtools

## Building & Testing Commands

build: ## Build - Build all components for current platform
	@echo "$(BLUE)Building all components...$(NC)"
	@mkdir -p bin
	@echo "Building controller..."
	@cd controller && go build $(BUILD_FLAGS) -o ../bin/controller .
	@echo "Building agent..."
	@cd agent && go build $(BUILD_FLAGS) -o ../bin/agent .
	@echo "Building CLI..."
	@cd cli && go build $(BUILD_FLAGS) -o ../bin/chaoslabs-cli .
	@echo "Building frontend..."
	@cd dashboard-v2 && npm run build
	@echo "$(GREEN)✓ Build complete! Binaries in ./bin/$(NC)"

build-cross: ## Build - Cross-compile for multiple platforms
	@echo "$(BLUE)Cross-compiling for multiple platforms...$(NC)"
	@mkdir -p bin/cross
	@for os in linux darwin windows; do \
		for arch in amd64 arm64; do \
			if [ "$$os" = "windows" ] && [ "$$arch" = "arm64" ]; then continue; fi; \
			echo "Building for $$os/$$arch..."; \
			for component in controller agent cli; do \
				ext=""; \
				if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
				output="bin/cross/$$component-$$os-$$arch$$ext"; \
				cd $$component && GOOS=$$os GOARCH=$$arch go build $(BUILD_FLAGS) -o ../$$output . && cd ..; \
			done; \
		done; \
	done
	@echo "$(GREEN)✓ Cross-compilation complete! Binaries in ./bin/cross/$(NC)"

test: ## Test - Run all tests with coverage
	@echo "$(BLUE)Running all tests...$(NC)"
	@mkdir -p coverage
	@echo "Testing controller..."
	@cd controller && go test -race -coverprofile=../coverage/controller.out -covermode=atomic ./...
	@echo "Testing agent..."
	@cd agent && go test -race -coverprofile=../coverage/agent.out -covermode=atomic ./...
	@echo "Testing CLI..."
	@cd cli && go test -race -coverprofile=../coverage/cli.out -covermode=atomic ./...
	@echo "Testing frontend..."
	@cd dashboard-v2 && npm test -- --coverage --watchAll=false
	@echo "$(GREEN)✓ All tests passed!$(NC)"

test-unit: ## Test - Run only unit tests (fast)
	@echo "$(BLUE)Running unit tests...$(NC)"
	@cd controller && go test -short ./...
	@cd agent && go test -short ./...
	@cd cli && go test -short ./...

test-integration: ## Test - Run integration tests
	@echo "$(BLUE)Running integration tests...$(NC)"
	@go test -tags=integration -v ./tests/integration/...

test-e2e: ## Test - Run end-to-end tests
	@echo "$(BLUE)Running end-to-end tests...$(NC)"
	@docker-compose -f infrastructure/docker-compose.test.yml up --build --abort-on-container-exit
	@docker-compose -f infrastructure/docker-compose.test.yml down -v

test-coverage: ## Test - Generate detailed coverage report
	@echo "$(BLUE)Generating coverage report...$(NC)"
	@mkdir -p coverage/html
	@go tool cover -html=coverage/controller.out -o coverage/html/controller.html
	@go tool cover -html=coverage/agent.out -o coverage/html/agent.html
	@go tool cover -html=coverage/cli.out -o coverage/html/cli.html
	@echo "$(GREEN)✓ Coverage reports generated in ./coverage/html/$(NC)"

bench: ## Test - Run benchmarks
	@echo "$(BLUE)Running benchmarks...$(NC)"
	@mkdir -p benchmarks
	@cd controller && go test -bench=. -benchmem -count=3 > ../benchmarks/controller.txt
	@cd agent && go test -bench=. -benchmem -count=3 > ../benchmarks/agent.txt
	@cd cli && go test -bench=. -benchmem -count=3 > ../benchmarks/cli.txt

## Quality & Analysis Commands

lint: ## Quality - Run linting on all code
	@echo "$(BLUE)Running linters...$(NC)"
	@echo "Linting Go code..."
	@golangci-lint run --config .golangci.yml
	@echo "Linting frontend code..."
	@cd dashboard-v2 && npm run lint
	@echo "$(GREEN)✓ All linting passed!$(NC)"

format: ## Quality - Format all code
	@echo "$(BLUE)Formatting code...$(NC)"
	@echo "Formatting Go code..."
	@gofmt -w .
	@goimports -w .
	@echo "Formatting frontend code..."
	@cd dashboard-v2 && npm run format
	@echo "$(GREEN)✓ Code formatting complete!$(NC)"

vet: ## Quality - Run Go vet
	@echo "$(BLUE)Running go vet...$(NC)"
	@go vet ./...

security-scan: ## Quality - Run security scans
	@echo "$(BLUE)Running security scans...$(NC)"
	@echo "Scanning for vulnerabilities..."
	@govulncheck ./...
	@echo "Auditing frontend dependencies..."
	@cd dashboard-v2 && npm audit --audit-level=moderate
	@echo "$(GREEN)✓ Security scan complete!$(NC)"

dependency-check: ## Quality - Check for outdated dependencies
	@echo "$(BLUE)Checking dependencies...$(NC)"
	@echo "Go modules:"
	@go list -u -m all
	@echo ""
	@echo "Frontend dependencies:"
	@cd dashboard-v2 && npm outdated || true

## Docker & Deployment Commands

docker-dev: ## Docker - Build development Docker images
	@echo "$(BLUE)Building development Docker images...$(NC)"
	@docker-compose -f infrastructure/docker-compose.dev.yml build

docker-build: ## Docker - Build production Docker images
	@echo "$(BLUE)Building production Docker images...$(NC)"
	@docker build -f infrastructure/Dockerfile.controller.optimized -t $(PROJECT_NAME)/controller:$(VERSION) .
	@docker build -f infrastructure/Dockerfile.agent.optimized -t $(PROJECT_NAME)/agent:$(VERSION) .
	@docker build -f infrastructure/Dockerfile.dashboard.optimized -t $(PROJECT_NAME)/dashboard:$(VERSION) ./dashboard-v2
	@echo "$(GREEN)✓ Production images built with tag: $(VERSION)$(NC)"

docker-push: ## Docker - Push images to registry
	@echo "$(BLUE)Pushing Docker images...$(NC)"
	@docker push $(PROJECT_NAME)/controller:$(VERSION)
	@docker push $(PROJECT_NAME)/agent:$(VERSION)
	@docker push $(PROJECT_NAME)/dashboard:$(VERSION)

docker-scan: ## Docker - Scan images for vulnerabilities
	@echo "$(BLUE)Scanning Docker images...$(NC)"
	@docker scout cves $(PROJECT_NAME)/controller:$(VERSION) || echo "Docker Scout not available"
	@docker scout cves $(PROJECT_NAME)/agent:$(VERSION) || echo "Docker Scout not available"
	@docker scout cves $(PROJECT_NAME)/dashboard:$(VERSION) || echo "Docker Scout not available"

## Performance & Analysis Commands

perf-test: ## Analysis - Run performance tests
	@echo "$(BLUE)Running performance tests...$(NC)"
	@k6 run tests/performance/load-test.js
	@k6 run tests/performance/stress-test.js

perf-report: ## Analysis - Generate CI/CD performance report
	@echo "$(BLUE)Generating performance report...$(NC)"
ifeq ($(OS),Windows_NT)
	@powershell -ExecutionPolicy Bypass -File infrastructure/performance-report.ps1
else
	@chmod +x infrastructure/performance-report.sh
	@infrastructure/performance-report.sh
endif

cache-warm: ## Analysis - Warm up caches for better CI/CD performance
	@echo "$(BLUE)Warming up caches...$(NC)"
ifeq ($(OS),Windows_NT)
	@powershell -ExecutionPolicy Bypass -File infrastructure/cache-warming.ps1
else
	@chmod +x infrastructure/cache-warming.sh
	@infrastructure/cache-warming.sh
endif

profile: ## Analysis - Generate CPU and memory profiles
	@echo "$(BLUE)Generating profiles...$(NC)"
	@mkdir -p profiles
	@cd controller && go test -cpuprofile=../profiles/controller-cpu.prof -memprofile=../profiles/controller-mem.prof -bench=.
	@cd agent && go test -cpuprofile=../profiles/agent-cpu.prof -memprofile=../profiles/agent-mem.prof -bench=.

## Monitoring & Debugging Commands

logs-controller: ## Debug - Show controller logs
	@docker-compose -f infrastructure/docker-compose.dev.yml logs -f controller

logs-agent: ## Debug - Show agent logs
	@docker-compose -f infrastructure/docker-compose.dev.yml logs -f agent

logs-all: ## Debug - Show all service logs
	@docker-compose -f infrastructure/docker-compose.dev.yml logs -f

db-shell: ## Debug - Connect to Redis shell
	@docker-compose -f infrastructure/docker-compose.dev.yml exec redis redis-cli

monitoring: ## Debug - Open monitoring dashboards
	@echo "$(BLUE)Opening monitoring dashboards...$(NC)"
	@echo "Grafana: http://localhost:3001 (admin/chaoslabs)"
	@echo "Prometheus: http://localhost:9090"
	@echo "Jaeger: http://localhost:16686"
	@echo "Dashboard: http://localhost:3000"
	@if command -v open >/dev/null 2>&1; then \
		open http://localhost:3001; \
	elif command -v xdg-open >/dev/null 2>&1; then \
		xdg-open http://localhost:3001; \
	fi

## Deployment Commands

deploy-staging: ## Deploy - Deploy to staging environment
	@echo "$(BLUE)Deploying to staging...$(NC)"
	@kubectl apply -f infrastructure/k8s/ --namespace=chaoslabs-staging

deploy-prod: ## Deploy - Deploy to production environment
	@echo "$(YELLOW)Deploying to production...$(NC)"
	@read -p "Are you sure you want to deploy to production? [y/N] " -n 1 -r; \
	echo; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		kubectl apply -f infrastructure/k8s/ --namespace=chaoslabs-production; \
	else \
		echo "Deployment cancelled."; \
	fi

rollback: ## Deploy - Rollback to previous version
	@echo "$(YELLOW)Rolling back deployment...$(NC)"
	@kubectl rollout undo deployment/controller --namespace=chaoslabs-production
	@kubectl rollout undo deployment/agent --namespace=chaoslabs-production

## Cleanup Commands

clean: ## Clean up build artifacts and temporary files
	@echo "$(BLUE)Cleaning up...$(NC)"
	@rm -rf bin/
	@rm -rf coverage/
	@rm -rf benchmarks/
	@rm -rf profiles/
	@rm -rf tmp/
	@cd dashboard-v2 && rm -rf dist/ node_modules/.cache
	@go clean -cache -testcache -modcache
	@echo "$(GREEN)✓ Cleanup complete!$(NC)"

clean-docker: ## Clean up Docker resources
	@echo "$(BLUE)Cleaning Docker resources...$(NC)"
	@docker-compose -f infrastructure/docker-compose.dev.yml down -v --remove-orphans
	@docker system prune -f
	@echo "$(GREEN)✓ Docker cleanup complete!$(NC)"

clean-all: clean clean-docker ## Clean up everything

## CI/CD Commands

ci-lint: ## CI - Run linting (optimized for CI)
	@golangci-lint run --out-format=github-actions --issues-exit-code=1
	@cd dashboard-v2 && npm run lint -- --format=unix

ci-test: ## CI - Run tests (optimized for CI)
	@go test -race -coverprofile=coverage.out -covermode=atomic ./...
	@cd dashboard-v2 && npm test -- --coverage --watchAll=false --reporters=default --reporters=jest-junit

ci-build: ## CI - Build for CI/CD
	@mkdir -p artifacts
	@$(MAKE) build-cross
	@tar -czf artifacts/binaries-$(VERSION).tar.gz -C bin/cross .
	@cd dashboard-v2 && npm run build && tar -czf ../artifacts/frontend-$(VERSION).tar.gz -C dist .

## Development Utilities

check-all: ## Utility - Run all quality checks
	@echo "$(BLUE)Running all quality checks...$(NC)"
ifeq ($(OS),Windows_NT)
	@powershell -ExecutionPolicy Bypass -File scripts/check-all.ps1
else
	@chmod +x scripts/check-all.sh
	@scripts/check-all.sh
endif

reset-dev: ## Utility - Reset development environment
	@echo "$(BLUE)Resetting development environment...$(NC)"
	@chmod +x scripts/reset-dev.sh
	@scripts/reset-dev.sh

install-tools: ## Utility - Install required development tools
	@echo "$(BLUE)Installing development tools...$(NC)"
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/go-delve/delve/cmd/dlv@latest
	@go install golang.org/x/tools/cmd/goimports@latest
	@go install golang.org/x/vuln/cmd/govulncheck@latest
	@go install github.com/air-verse/air@latest
	@echo "$(GREEN)✓ Development tools installed!$(NC)"

version: ## Utility - Show version information
	@echo "Project: $(PROJECT_NAME)"
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Go Version: $(shell go version)"
	@echo "Platform: $(GOOS)/$(GOARCH)"

## Documentation

docs-serve: ## Docs - Serve documentation locally
	@cd docs && npm run serve

docs-build: ## Docs - Build documentation
	@cd docs && npm run build

docs-deploy: ## Docs - Deploy documentation to GitHub Pages
	@cd docs && npm run deploy

# Load test targets if k6 is available
ifeq ($(OS),Windows_NT)
K6_EXISTS := $(shell powershell -Command "if (Get-Command k6 -ErrorAction SilentlyContinue) { Write-Output 'yes' } else { Write-Output 'no' }")
else
K6_EXISTS := $(shell which k6 2>/dev/null)
endif

ifneq ($(K6_EXISTS),)
ifneq ($(K6_EXISTS),no)
load-test-light: ## Load Test - Light load test (100 VUs)
	@k6 run --vus 100 --duration 30s tests/performance/load-test.js

load-test-medium: ## Load Test - Medium load test (500 VUs)
	@k6 run --vus 500 --duration 2m tests/performance/load-test.js

load-test-heavy: ## Load Test - Heavy load test (1000 VUs)
	@k6 run --vus 1000 --duration 5m tests/performance/load-test.js
endif
endif

# Database targets if available
ifneq (,$(shell docker-compose -f infrastructure/docker-compose.dev.yml ps -q mongodb 2>/dev/null))
db-backup: ## Database - Backup MongoDB
	@docker-compose -f infrastructure/docker-compose.dev.yml exec mongodb mongodump --out /tmp/backup
	@docker cp $$(docker-compose -f infrastructure/docker-compose.dev.yml ps -q mongodb):/tmp/backup ./backup-$(shell date +%Y%m%d_%H%M%S)

db-restore: ## Database - Restore MongoDB (requires BACKUP_DIR)
	@if [ -z "$(BACKUP_DIR)" ]; then echo "Usage: make db-restore BACKUP_DIR=./backup-20231201_120000"; exit 1; fi
	@docker cp $(BACKUP_DIR) $$(docker-compose -f infrastructure/docker-compose.dev.yml ps -q mongodb):/tmp/restore
	@docker-compose -f infrastructure/docker-compose.dev.yml exec mongodb mongorestore /tmp/restore
endif
