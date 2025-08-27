# ChaosLabs Agent Makefile
# Provides targets for building, testing, and running the agent

.PHONY: help build test clean preflight doctor run-benchmarks

# Default target
help:
	@echo "ChaosLabs Agent - Available targets:"
	@echo "  build           - Build the agent binary"
	@echo "  test            - Run all tests"
	@echo "  test-preflight  - Run preflight system tests only"
	@echo "  test-benchmark  - Run benchmark tests"
	@echo "  clean           - Clean build artifacts"
	@echo "  preflight       - Run preflight checks"
	@echo "  doctor          - Build and run chaoslabs doctor CLI"
	@echo "  run-benchmarks  - Run performance benchmarks"
	@echo "  install-deps    - Install Go dependencies"
	@echo "  lint            - Run code linting"
	@echo "  format          - Format Go code"

# Build the agent
build:
	@echo "Building ChaosLabs Agent..."
	go build -o bin/chaoslabs-agent .
	@echo "Build complete: bin/chaoslabs-agent"

# Run all tests
test:
	@echo "Running all tests..."
	go test -v ./...
	@echo "Tests complete"

# Run preflight tests only
test-preflight:
	@echo "Running preflight system tests..."
	go test -v -run "TestPreflight" ./...
	@echo "Preflight tests complete"

# Run benchmark tests
test-benchmark:
	@echo "Running benchmark tests..."
	go test -v -bench=. -benchmem ./...
	@echo "Benchmark tests complete"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -rf coverage/
	go clean -cache -testcache
	@echo "Clean complete"

# Run preflight checks
preflight: build
	@echo "Running preflight checks..."
	./bin/chaoslabs-agent --preflight

# Build and run chaoslabs doctor CLI
doctor:
	@echo "Building chaoslabs doctor CLI..."
	cd cmd/doctor && go build -o ../../bin/chaoslabs-doctor .
	@echo "Build complete: bin/chaoslabs-doctor"
	@echo "Running preflight checks..."
	./bin/chaoslabs-doctor

# Run performance benchmarks
run-benchmarks: build
	@echo "Running performance benchmarks..."
	cd ../bench && ./run_benchmarks.sh

# Install Go dependencies
install-deps:
	@echo "Installing Go dependencies..."
	go mod download
	go mod tidy
	@echo "Dependencies installed"

# Run code linting
lint:
	@echo "Running code linting..."
	golangci-lint run ./...
	@echo "Linting complete"

# Format Go code
format:
	@echo "Formatting Go code..."
	go fmt ./...
	@echo "Formatting complete"

# Run with race detection
test-race:
	@echo "Running tests with race detection..."
	go test -race -v ./...
	@echo "Race detection tests complete"

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	mkdir -p coverage
	go test -v -coverprofile=coverage/coverage.out ./...
	go tool cover -html=coverage/coverage.out -o coverage/coverage.html
	@echo "Coverage report generated: coverage/coverage.html"

# Build for different architectures
build-linux-amd64:
	@echo "Building for Linux AMD64..."
	GOOS=linux GOARCH=amd64 go build -o bin/chaoslabs-agent-linux-amd64 .

build-linux-arm64:
	@echo "Building for Linux ARM64..."
	GOOS=linux GOARCH=arm64 go build -o bin/chaoslabs-agent-linux-arm64 .

build-all: build-linux-amd64 build-linux-arm64
	@echo "Build for all architectures complete"

# Docker targets
docker-build:
	@echo "Building Docker image..."
	docker build -t chaoslabs-agent:latest -f ../infrastructure/Dockerfile.agent .
	@echo "Docker build complete"

docker-run:
	@echo "Running Docker container..."
	docker run --rm --cap-add=NET_ADMIN --cap-add=SYS_ADMIN \
		-v /proc:/proc -v /sys:/sys -v /dev:/dev \
		chaoslabs-agent:latest

# Development targets
dev-run: build
	@echo "Running agent in development mode..."
	./bin/chaoslabs-agent --dev

# Quick check - run basic tests and build
quick-check: test-preflight build
	@echo "Quick check complete"

# Full check - run all tests, build, and lint
full-check: install-deps lint test build
	@echo "Full check complete"

# CI target - run all checks for continuous integration
ci: install-deps lint test test-race test-coverage build
	@echo "CI checks complete"

# Release target - build release binaries
release: clean build-all
	@echo "Creating release archive..."
	tar -czf chaoslabs-agent-$(shell git describe --tags --always).tar.gz bin/
	@echo "Release archive created"

# Help for specific target
help-%:
	@echo "Target: $*"
	@echo "Usage: make $*"
	@echo "Description: $(shell grep -A1 "^$*:" Makefile | tail -1 | sed 's/^[[:space:]]*@echo "//' | sed 's/"$$//')"
