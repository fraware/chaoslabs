#!/bin/bash

# ChaosLabs Development Environment Setup Script
# This script sets up a complete development environment

set -e

echo "🚀 Setting up ChaosLabs development environment..."

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if running in workspace
if [ ! -f "go.mod" ]; then
    print_error "Please run this script from the ChaosLabs workspace root"
    exit 1
fi

# Setup Git hooks for development
print_status "Setting up Git hooks..."
if [ ! -d ".git/hooks" ]; then
    mkdir -p .git/hooks
fi

# Pre-commit hook
cat > .git/hooks/pre-commit << 'EOF'
#!/bin/bash
# ChaosLabs pre-commit hook

set -e

echo "Running pre-commit checks..."

# Check Go formatting
echo "Checking Go formatting..."
if [ -n "$(gofmt -l .)" ]; then
    echo "Go code is not formatted. Please run 'gofmt -w .'"
    exit 1
fi

# Run Go linting
echo "Running Go linting..."
golangci-lint run

# Run Go tests
echo "Running Go tests..."
go test ./... -short

# Check frontend formatting (if changed)
if git diff --cached --name-only | grep -q "dashboard-v2/"; then
    echo "Checking frontend code..."
    cd dashboard-v2
    npm run lint
    npm run type-check
    cd ..
fi

echo "Pre-commit checks passed!"
EOF

chmod +x .git/hooks/pre-commit

# Setup Go development environment
print_status "Setting up Go development environment..."

# Download dependencies
go mod download
go mod tidy

# Install development tools if not already installed
tools=(
    "github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
    "github.com/go-delve/delve/cmd/dlv@latest"
    "golang.org/x/tools/cmd/goimports@latest"
    "github.com/air-verse/air@latest"
    "golang.org/x/vuln/cmd/govulncheck@latest"
)

for tool in "${tools[@]}"; do
    tool_name=$(basename ${tool%@*})
    if ! command -v $tool_name &> /dev/null; then
        print_status "Installing $tool_name..."
        go install $tool
    else
        print_success "$tool_name already installed"
    fi
done

# Setup frontend development environment
if [ -d "dashboard-v2" ]; then
    print_status "Setting up frontend development environment..."
    cd dashboard-v2
    
    if [ ! -d "node_modules" ]; then
        print_status "Installing Node.js dependencies..."
        npm ci
    else
        print_success "Node.js dependencies already installed"
    fi
    
    cd ..
fi

# Create development configuration files
print_status "Creating development configuration files..."

# Air configuration for Go hot reload
if [ ! -f ".air.toml" ]; then
cat > .air.toml << 'EOF'
root = "."
testdata_dir = "testdata"
tmp_dir = "tmp"

[build]
  args_bin = []
  bin = "./tmp/main"
  cmd = "go build -o ./tmp/main ./controller/"
  delay = 1000
  exclude_dir = ["assets", "tmp", "vendor", "testdata", "node_modules", "dashboard-v2"]
  exclude_file = []
  exclude_regex = ["_test.go"]
  exclude_unchanged = false
  follow_symlink = false
  full_bin = ""
  include_dir = []
  include_ext = ["go", "tpl", "tmpl", "html"]
  kill_delay = "0s"
  log = "build-errors.log"
  send_interrupt = false
  stop_on_root = false

[color]
  app = ""
  build = "yellow"
  main = "magenta"
  runner = "green"
  watcher = "cyan"

[log]
  time = false

[misc]
  clean_on_exit = false

[screen]
  clear_on_rebuild = false
EOF
fi

# Makefile for common development tasks
if [ ! -f "Makefile" ]; then
cat > Makefile << 'EOF'
.PHONY: help dev build test lint clean docker-dev docker-build

# Default target
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

dev: ## Start development environment
	docker-compose -f infrastructure/docker-compose.dev.yml up

dev-controller: ## Start controller with hot reload
	air -c .air.toml

dev-frontend: ## Start frontend development server
	cd dashboard-v2 && npm run dev

build: ## Build all components
	go build -o bin/controller ./controller/
	go build -o bin/agent ./agent/
	go build -o bin/cli ./cli/
	cd dashboard-v2 && npm run build

test: ## Run all tests
	go test ./... -race -coverprofile=coverage.out
	cd dashboard-v2 && npm test

test-integration: ## Run integration tests
	go test -tags=integration ./tests/integration/...

lint: ## Run linting
	golangci-lint run
	cd dashboard-v2 && npm run lint

format: ## Format code
	gofmt -w .
	goimports -w .
	cd dashboard-v2 && npm run format

clean: ## Clean build artifacts
	rm -rf bin/
	rm -rf tmp/
	rm -rf coverage.out
	cd dashboard-v2 && rm -rf dist/

docker-dev: ## Build development Docker images
	docker-compose -f infrastructure/docker-compose.dev.yml build

docker-build: ## Build production Docker images
	docker build -f infrastructure/Dockerfile.controller.optimized -t chaoslabs/controller:latest .
	docker build -f infrastructure/Dockerfile.agent.optimized -t chaoslabs/agent:latest .

security-scan: ## Run security scans
	govulncheck ./...
	cd dashboard-v2 && npm audit

performance-test: ## Run performance tests
	k6 run tests/performance/load-test.js

deploy-staging: ## Deploy to staging
	kubectl apply -f infrastructure/k8s/ --namespace=chaoslabs-staging

logs-controller: ## Show controller logs
	docker-compose -f infrastructure/docker-compose.dev.yml logs -f controller

logs-agent: ## Show agent logs
	docker-compose -f infrastructure/docker-compose.dev.yml logs -f agent

db-shell: ## Connect to Redis shell
	docker-compose -f infrastructure/docker-compose.dev.yml exec redis redis-cli

monitoring: ## Open monitoring dashboards
	@echo "Opening monitoring dashboards..."
	@echo "Grafana: http://localhost:3001 (admin/chaoslabs)"
	@echo "Prometheus: http://localhost:9090"
	@echo "Jaeger: http://localhost:16686"
EOF
fi

# VS Code configuration
if [ ! -d ".vscode" ]; then
    mkdir -p .vscode
    
    # VS Code settings
    cat > .vscode/settings.json << 'EOF'
{
    "go.toolsManagement.checkForUpdates": "local",
    "go.useLanguageServer": true,
    "go.gopath": "",
    "go.goroot": "",
    "go.lintTool": "golangci-lint",
    "go.lintFlags": [
        "--fast"
    ],
    "go.formatTool": "goimports",
    "go.testFlags": ["-v", "-race"],
    "go.testTimeout": "30s",
    "go.coverOnSave": true,
    "go.coverMode": "atomic",
    "files.exclude": {
        "**/node_modules": true,
        "**/tmp": true,
        "**/bin": true,
        "**/.git": true
    },
    "search.exclude": {
        "**/node_modules": true,
        "**/tmp": true,
        "**/bin": true,
        "**/vendor": true
    },
    "typescript.preferences.quoteStyle": "single",
    "editor.formatOnSave": true,
    "editor.codeActionsOnSave": {
        "source.organizeImports": true
    }
}
EOF

    # VS Code extensions recommendations
    cat > .vscode/extensions.json << 'EOF'
{
    "recommendations": [
        "golang.go",
        "ms-vscode.vscode-typescript-next",
        "bradlc.vscode-tailwindcss",
        "esbenp.prettier-vscode",
        "ms-vscode.vscode-eslint",
        "ms-kubernetes-tools.vscode-kubernetes-tools",
        "ms-vscode.vscode-docker",
        "github.vscode-pull-request-github",
        "streetsidesoftware.code-spell-checker"
    ]
}
EOF

    # VS Code launch configuration
    cat > .vscode/launch.json << 'EOF'
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Debug Controller",
            "type": "go",
            "request": "launch",
            "mode": "debug",
            "program": "${workspaceFolder}/controller",
            "env": {
                "LOG_LEVEL": "debug",
                "REDIS_URL": "redis://localhost:6379",
                "NATS_URL": "nats://localhost:4222"
            },
            "args": []
        },
        {
            "name": "Debug Agent",
            "type": "go",
            "request": "launch",
            "mode": "debug",
            "program": "${workspaceFolder}/agent",
            "env": {
                "LOG_LEVEL": "debug"
            },
            "args": []
        },
        {
            "name": "Debug Tests",
            "type": "go",
            "request": "launch",
            "mode": "test",
            "program": "${workspaceFolder}",
            "args": [
                "-test.v"
            ]
        }
    ]
}
EOF

fi

# Create environment configuration
if [ ! -f ".env.development" ]; then
    cat > .env.development << 'EOF'
# ChaosLabs Development Environment Configuration

# Logging
LOG_LEVEL=debug
LOG_FORMAT=text

# Services
REDIS_URL=redis://localhost:6379
NATS_URL=nats://localhost:4222
JAEGER_ENDPOINT=http://localhost:14268/api/traces

# Controller
CONTROLLER_PORT=8080
CONTROLLER_NODE_ID=controller-dev-1

# Agent
AGENT_PORT=9090
AGENT_NODE_ID=agent-dev-1

# Dashboard
DASHBOARD_PORT=3000
VITE_API_URL=http://localhost:8080
VITE_WS_URL=ws://localhost:8080/ws

# Database
MONGO_URI=mongodb://chaoslabs:chaoslabs@localhost:27017/chaoslabs

# Security (development only)
JWT_SECRET=dev-secret-key-not-for-production
API_KEY=dev-api-key

# Feature flags
ENABLE_METRICS=true
ENABLE_TRACING=true
ENABLE_PROFILING=true
EOF
fi

# Create useful development scripts
mkdir -p scripts

# Script to reset development environment
cat > scripts/reset-dev.sh << 'EOF'
#!/bin/bash
echo "🔄 Resetting development environment..."

# Stop all containers
docker-compose -f infrastructure/docker-compose.dev.yml down -v

# Remove build artifacts
make clean

# Rebuild everything
docker-compose -f infrastructure/docker-compose.dev.yml build --no-cache

echo "✅ Development environment reset complete!"
EOF

chmod +x scripts/reset-dev.sh

# Script to run all checks
cat > scripts/check-all.sh << 'EOF'
#!/bin/bash
echo "🔍 Running all quality checks..."

set -e

echo "📝 Running Go linting..."
golangci-lint run

echo "🧪 Running Go tests..."
go test ./... -race -coverprofile=coverage.out

echo "🔒 Running security scan..."
govulncheck ./...

if [ -d "dashboard-v2" ]; then
    echo "🎨 Running frontend checks..."
    cd dashboard-v2
    npm run lint
    npm run type-check
    npm test
    cd ..
fi

echo "✅ All checks passed!"
EOF

chmod +x scripts/check-all.sh

# Setup completion
print_success "Development environment setup complete!"
print_status "Next steps:"
echo "  1. Run 'make dev' to start the development environment"
echo "  2. Run 'make help' to see available commands"
echo "  3. Open the project in VS Code for the best development experience"
echo ""
print_status "Useful commands:"
echo "  • make dev-controller    - Start controller with hot reload"
echo "  • make dev-frontend      - Start frontend development server"
echo "  • make test              - Run all tests"
echo "  • make lint              - Run linting"
echo "  • scripts/check-all.sh   - Run all quality checks"
echo ""
print_status "Monitoring dashboards (after running 'make dev'):"
echo "  • Grafana: http://localhost:3001 (admin/chaoslabs)"
echo "  • Prometheus: http://localhost:9090"
echo "  • Jaeger: http://localhost:16686"
echo "  • Dashboard: http://localhost:3000"
