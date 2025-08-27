#!/bin/bash

# Cache Warming Script for CI/CD Pipeline Optimization
# This script pre-warms various caches to improve CI/CD performance

set -e

echo "🔥 Warming up caches for faster CI/CD..."

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

print_status() {
    echo -e "${BLUE}[CACHE]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Function to check if Docker is available
check_docker() {
    if ! command -v docker &> /dev/null; then
        print_warning "Docker not found, skipping Docker cache warming"
        return 1
    fi
    return 0
}

# Function to check if Go is available
check_go() {
    if ! command -v go &> /dev/null; then
        print_warning "Go not found, skipping Go cache warming"
        return 1
    fi
    return 0
}

# Function to check if Node.js is available
check_node() {
    if ! command -v npm &> /dev/null; then
        print_warning "Node.js/npm not found, skipping Node cache warming"
        return 1
    fi
    return 0
}

# Warm Go module cache
warm_go_cache() {
    if check_go; then
        print_status "Warming Go module cache..."
        
        # Download dependencies for all modules
        for module in controller agent cli; do
            if [ -f "$module/go.mod" ]; then
                print_status "Downloading dependencies for $module..."
                cd $module
                go mod download
                go mod verify
                cd ..
            fi
        done
        
        # Pre-compile standard library for common targets
        print_status "Pre-compiling Go standard library..."
        GOOS=linux GOARCH=amd64 go install -a std
        GOOS=darwin GOARCH=amd64 go install -a std
        
        # Install common development tools
        print_status "Installing Go development tools..."
        go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
        go install github.com/go-delve/delve/cmd/dlv@latest
        go install golang.org/x/tools/cmd/goimports@latest
        go install golang.org/x/vuln/cmd/govulncheck@latest
        
        print_success "Go cache warmed successfully"
    fi
}

# Warm Node.js cache
warm_node_cache() {
    if check_node; then
        print_status "Warming Node.js cache..."
        
        # Cache dashboard dependencies
        if [ -f "dashboard-v2/package.json" ]; then
            print_status "Caching dashboard dependencies..."
            cd dashboard-v2
            npm ci --prefer-offline --no-audit
            
            # Pre-build commonly used packages
            npm run build || print_warning "Dashboard build failed during cache warming"
            cd ..
        fi
        
        # Cache documentation dependencies
        if [ -f "docs/package.json" ]; then
            print_status "Caching documentation dependencies..."
            cd docs
            npm ci --prefer-offline --no-audit
            cd ..
        fi
        
        print_success "Node.js cache warmed successfully"
    fi
}

# Warm Docker build cache
warm_docker_cache() {
    if check_docker; then
        print_status "Warming Docker build cache..."
        
        # Build base images with cache
        print_status "Building Go build cache image..."
        docker build \
            --target build-cache \
            --cache-from chaoslabs/controller:build-cache \
            -t chaoslabs/controller:build-cache \
            -f infrastructure/Dockerfile.controller.optimized \
            .
        
        # Build development images
        print_status "Building development images..."
        docker build \
            --target development \
            --cache-from chaoslabs/controller:build-cache \
            --cache-from chaoslabs/controller:development \
            -t chaoslabs/controller:development \
            -f infrastructure/Dockerfile.controller.optimized \
            .
        
        # Pull commonly used base images
        print_status "Pulling common base images..."
        docker pull golang:1.21-alpine
        docker pull node:18-alpine
        docker pull alpine:3.18
        docker pull gcr.io/distroless/static-debian11:nonroot
        docker pull redis:7-alpine
        docker pull nats:2.10-alpine
        docker pull prom/prometheus:latest
        docker pull grafana/grafana:latest
        
        print_success "Docker cache warmed successfully"
    fi
}

# Warm GitHub Actions cache
warm_github_cache() {
    print_status "Setting up GitHub Actions cache optimization..."
    
    # Create cache key files for better cache hits
    find . -name "go.mod" -o -name "go.sum" | sort | xargs cat | sha256sum > .github-cache-go.key
    find . -name "package.json" -o -name "package-lock.json" | sort | xargs cat | sha256sum > .github-cache-node.key
    find infrastructure/ -name "Dockerfile*" | sort | xargs cat | sha256sum > .github-cache-docker.key
    
    print_success "GitHub Actions cache keys generated"
}

# Warm test data cache
warm_test_cache() {
    print_status "Warming test data cache..."
    
    # Create test data directory
    mkdir -p tests/cache
    
    # Generate test data for performance tests
    if check_go; then
        print_status "Generating test data..."
        go run tests/generate-test-data.go || print_warning "Test data generation failed"
    fi
    
    print_success "Test cache warmed successfully"
}

# Pre-compile frequently used tools
warm_tools_cache() {
    print_status "Pre-compiling development tools..."
    
    if check_go; then
        # Tools that are commonly used in CI/CD
        tools=(
            "github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
            "golang.org/x/vuln/cmd/govulncheck@latest"
            "github.com/securecodewarrior/goat@latest"
            "mvdan.cc/gofumpt@latest"
            "golang.org/x/tools/cmd/goimports@latest"
        )
        
        for tool in "${tools[@]}"; do
            tool_name=$(basename ${tool%@*})
            if ! command -v $tool_name &> /dev/null; then
                print_status "Installing $tool_name..."
                go install $tool || print_warning "Failed to install $tool"
            fi
        done
    fi
    
    print_success "Tools cache warmed successfully"
}

# Generate performance benchmarks
warm_benchmark_cache() {
    print_status "Generating benchmark baselines..."
    
    if check_go; then
        # Run benchmarks to establish baselines
        for module in controller agent cli; do
            if [ -d "$module" ]; then
                print_status "Running benchmarks for $module..."
                cd $module
                go test -bench=. -benchmem -count=3 > ../tests/benchmarks/$module-baseline.txt 2>/dev/null || true
                cd ..
            fi
        done
    fi
    
    print_success "Benchmark cache warmed successfully"
}

# Main execution
main() {
    echo "🚀 Starting cache warming process..."
    echo "This may take a few minutes but will significantly speed up future builds."
    echo ""
    
    # Create necessary directories
    mkdir -p tests/cache
    mkdir -p tests/benchmarks
    mkdir -p .cache
    
    # Run cache warming functions
    warm_go_cache
    warm_node_cache
    warm_docker_cache
    warm_github_cache
    warm_test_cache
    warm_tools_cache
    warm_benchmark_cache
    
    # Generate cache report
    echo ""
    print_success "Cache warming completed!"
    echo ""
    echo "📊 Cache Report:"
    echo "================"
    
    # Go cache size
    if check_go; then
        go_cache_size=$(du -sh $(go env GOCACHE) 2>/dev/null | cut -f1 || echo "Unknown")
        go_mod_size=$(du -sh $(go env GOMODCACHE) 2>/dev/null | cut -f1 || echo "Unknown")
        echo "Go build cache: $go_cache_size"
        echo "Go module cache: $go_mod_size"
    fi
    
    # Node cache size
    if check_node; then
        npm_cache_size=$(du -sh ~/.npm 2>/dev/null | cut -f1 || echo "Unknown")
        echo "npm cache: $npm_cache_size"
    fi
    
    # Docker cache size
    if check_docker; then
        docker_size=$(docker system df --format "table {{.Type}}\t{{.Size}}" | grep -E "Images|Build" | awk '{print $2}' | paste -sd+ | bc 2>/dev/null || echo "Unknown")
        echo "Docker cache: ${docker_size}B"
    fi
    
    echo ""
    echo "💡 Next time you run CI/CD or development builds, they should be significantly faster!"
    echo ""
    echo "🔧 To maintain optimal performance:"
    echo "   • Run this script monthly or when dependencies change significantly"
    echo "   • Use 'docker system prune' occasionally to clean up unused Docker cache"
    echo "   • Monitor cache sizes to ensure they don't grow too large"
}

# Run main function
main "$@"