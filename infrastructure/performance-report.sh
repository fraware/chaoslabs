#!/bin/bash

# CI/CD Performance Analysis and Reporting Script
# Analyzes build times, cache effectiveness, and generates optimization recommendations

set -e

# Configuration
REPORT_DIR="reports/performance"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_FILE="$REPORT_DIR/ci_performance_$TIMESTAMP.json"
HTML_REPORT="$REPORT_DIR/ci_performance_$TIMESTAMP.html"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

print_status() { echo -e "${BLUE}[PERF]${NC} $1"; }
print_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
print_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Initialize report structure
init_report() {
    mkdir -p "$REPORT_DIR"
    
    cat > "$REPORT_FILE" << EOF
{
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "version": "2.0",
  "metadata": {
    "git_commit": "$(git rev-parse HEAD 2>/dev/null || echo 'unknown')",
    "git_branch": "$(git branch --show-current 2>/dev/null || echo 'unknown')",
    "ci_system": "${CI:-local}",
    "runner_os": "$(uname -s)",
    "runner_arch": "$(uname -m)"
  },
  "build_performance": {},
  "cache_analysis": {},
  "test_performance": {},
  "docker_performance": {},
  "recommendations": []
}
EOF
}

# Analyze Go build performance
analyze_go_performance() {
    print_status "Analyzing Go build performance..."
    
    local go_data="{}"
    
    if command -v go &> /dev/null; then
        # Measure build times for each component
        for component in controller agent cli; do
            if [ -d "$component" ]; then
                print_status "Measuring $component build time..."
                
                # Clean build
                (cd "$component" && go clean -cache)
                local start_time=$(date +%s.%N)
                (cd "$component" && go build -o /tmp/test_build . 2>/dev/null) || true
                local end_time=$(date +%s.%N)
                local cold_build_time=$(echo "$end_time - $start_time" | bc)
                
                # Warm build
                local start_time=$(date +%s.%N)
                (cd "$component" && go build -o /tmp/test_build . 2>/dev/null) || true
                local end_time=$(date +%s.%N)
                local warm_build_time=$(echo "$end_time - $start_time" | bc)
                
                # Test compilation time
                local start_time=$(date +%s.%N)
                (cd "$component" && go test -c ./... 2>/dev/null) || true
                local end_time=$(date +%s.%N)
                local test_build_time=$(echo "$end_time - $start_time" | bc)
                
                # Cache statistics
                local cache_size=$(go env GOCACHE | xargs du -sb 2>/dev/null | cut -f1 || echo "0")
                local mod_cache_size=$(go env GOMODCACHE | xargs du -sb 2>/dev/null | cut -f1 || echo "0")
                
                go_data=$(echo "$go_data" | jq --arg component "$component" \
                    --arg cold_build "$cold_build_time" \
                    --arg warm_build "$warm_build_time" \
                    --arg test_build "$test_build_time" \
                    --arg cache_size "$cache_size" \
                    --arg mod_cache_size "$mod_cache_size" \
                    '. + {($component): {
                        "cold_build_seconds": ($cold_build | tonumber),
                        "warm_build_seconds": ($warm_build | tonumber),
                        "test_build_seconds": ($test_build | tonumber),
                        "cache_size_bytes": ($cache_size | tonumber),
                        "mod_cache_size_bytes": ($mod_cache_size | tonumber),
                        "cache_effectiveness": (($cold_build | tonumber) / ($warm_build | tonumber))
                    }}')
                
                rm -f /tmp/test_build
            fi
        done
        
        # Overall Go statistics
        local total_deps=$(find . -name go.mod -exec grep -c "require" {} \; | awk '{sum+=$1} END {print sum}' || echo "0")
        local go_version=$(go version | grep -o 'go[0-9.]*' || echo "unknown")
        
        go_data=$(echo "$go_data" | jq --arg total_deps "$total_deps" \
            --arg go_version "$go_version" \
            '. + {
                "summary": {
                    "go_version": $go_version,
                    "total_dependencies": ($total_deps | tonumber),
                    "modules_count": '"$(find . -name go.mod | wc -l)"'
                }
            }')
    fi
    
    # Update main report
    jq --argjson go_data "$go_data" '.build_performance.go = $go_data' "$REPORT_FILE" > /tmp/report.json && mv /tmp/report.json "$REPORT_FILE"
}

# Analyze Node.js build performance
analyze_node_performance() {
    print_status "Analyzing Node.js build performance..."
    
    local node_data="{}"
    
    if command -v npm &> /dev/null && [ -d "dashboard-v2" ]; then
        cd dashboard-v2
        
        # Measure install time
        rm -rf node_modules package-lock.json 2>/dev/null || true
        local start_time=$(date +%s.%N)
        npm install --silent 2>/dev/null || true
        local end_time=$(date +%s.%N)
        local install_time=$(echo "$end_time - $start_time" | bc)
        
        # Measure build time
        local start_time=$(date +%s.%N)
        npm run build --silent 2>/dev/null || true
        local end_time=$(date +%s.%N)
        local build_time=$(echo "$end_time - $start_time" | bc)
        
        # Cache statistics
        local npm_cache_size=$(npm config get cache | xargs du -sb 2>/dev/null | cut -f1 || echo "0")
        local node_modules_size=$(du -sb node_modules 2>/dev/null | cut -f1 || echo "0")
        
        # Bundle analysis
        local bundle_size=0
        if [ -d "dist" ]; then
            bundle_size=$(du -sb dist 2>/dev/null | cut -f1 || echo "0")
        fi
        
        # Dependencies count
        local deps_count=$(jq '.dependencies | length' package.json 2>/dev/null || echo "0")
        local dev_deps_count=$(jq '.devDependencies | length' package.json 2>/dev/null || echo "0")
        
        node_data=$(jq -n \
            --arg install_time "$install_time" \
            --arg build_time "$build_time" \
            --arg npm_cache_size "$npm_cache_size" \
            --arg node_modules_size "$node_modules_size" \
            --arg bundle_size "$bundle_size" \
            --arg deps_count "$deps_count" \
            --arg dev_deps_count "$dev_deps_count" \
            '{
                "install_seconds": ($install_time | tonumber),
                "build_seconds": ($build_time | tonumber),
                "npm_cache_size_bytes": ($npm_cache_size | tonumber),
                "node_modules_size_bytes": ($node_modules_size | tonumber),
                "bundle_size_bytes": ($bundle_size | tonumber),
                "dependencies_count": ($deps_count | tonumber),
                "dev_dependencies_count": ($dev_deps_count | tonumber),
                "node_version": "'"$(node --version 2>/dev/null || echo 'unknown')"'",
                "npm_version": "'"$(npm --version 2>/dev/null || echo 'unknown')"'"
            }')
        
        cd ..
    fi
    
    # Update main report
    jq --argjson node_data "$node_data" '.build_performance.node = $node_data' "$REPORT_FILE" > /tmp/report.json && mv /tmp/report.json "$REPORT_FILE"
}

# Analyze Docker build performance
analyze_docker_performance() {
    print_status "Analyzing Docker build performance..."
    
    local docker_data="{}"
    
    if command -v docker &> /dev/null; then
        # Measure Docker build times
        for component in controller agent dashboard; do
            if [ -f "infrastructure/Dockerfile.$component.optimized" ]; then
                print_status "Measuring Docker build time for $component..."
                
                # Cold build (no cache)
                docker builder prune -f > /dev/null 2>&1 || true
                local start_time=$(date +%s.%N)
                docker build \
                    -f "infrastructure/Dockerfile.$component.optimized" \
                    --target production \
                    -t "test-$component:latest" \
                    . > /dev/null 2>&1 || true
                local end_time=$(date +%s.%N)
                local cold_build_time=$(echo "$end_time - $start_time" | bc)
                
                # Warm build (with cache)
                local start_time=$(date +%s.%N)
                docker build \
                    -f "infrastructure/Dockerfile.$component.optimized" \
                    --target production \
                    -t "test-$component:cached" \
                    . > /dev/null 2>&1 || true
                local end_time=$(date +%s.%N)
                local warm_build_time=$(echo "$end_time - $start_time" | bc)
                
                # Image size
                local image_size=$(docker images --format "table {{.Size}}" "test-$component:latest" | tail -n 1 | numfmt --from=iec --to-unit=1 || echo "0")
                
                docker_data=$(echo "$docker_data" | jq --arg component "$component" \
                    --arg cold_build "$cold_build_time" \
                    --arg warm_build "$warm_build_time" \
                    --arg image_size "$image_size" \
                    '. + {($component): {
                        "cold_build_seconds": ($cold_build | tonumber),
                        "warm_build_seconds": ($warm_build | tonumber),
                        "image_size_bytes": ($image_size | tonumber),
                        "cache_effectiveness": (($cold_build | tonumber) / (($warm_build | tonumber) + 0.1))
                    }}')
                
                # Clean up test images
                docker rmi "test-$component:latest" "test-$component:cached" > /dev/null 2>&1 || true
            fi
        done
        
        # Docker system information
        local docker_version=$(docker version --format '{{.Server.Version}}' 2>/dev/null || echo "unknown")
        local buildkit_enabled=$(docker version --format '{{.Server.BuildkitVersion}}' 2>/dev/null | grep -q . && echo "true" || echo "false")
        local total_images=$(docker images -q | wc -l)
        local cache_usage=$(docker system df --format "table {{.Type}}\t{{.Size}}" | grep "Build Cache" | awk '{print $3}' | numfmt --from=iec --to-unit=1 2>/dev/null || echo "0")
        
        docker_data=$(echo "$docker_data" | jq --arg docker_version "$docker_version" \
            --arg buildkit_enabled "$buildkit_enabled" \
            --arg total_images "$total_images" \
            --arg cache_usage "$cache_usage" \
            '. + {
                "summary": {
                    "docker_version": $docker_version,
                    "buildkit_enabled": ($buildkit_enabled | test("true")),
                    "total_images": ($total_images | tonumber),
                    "cache_usage_bytes": ($cache_usage | tonumber)
                }
            }')
    fi
    
    # Update main report
    jq --argjson docker_data "$docker_data" '.docker_performance = $docker_data' "$REPORT_FILE" > /tmp/report.json && mv /tmp/report.json "$REPORT_FILE"
}

# Analyze test performance
analyze_test_performance() {
    print_status "Analyzing test performance..."
    
    local test_data="{}"
    
    if command -v go &> /dev/null; then
        for component in controller agent cli; do
            if [ -d "$component" ]; then
                print_status "Measuring test performance for $component..."
                
                cd "$component"
                
                # Run tests with timing
                local start_time=$(date +%s.%N)
                local test_output=$(go test -v ./... 2>&1 || true)
                local end_time=$(date +%s.%N)
                local test_duration=$(echo "$end_time - $start_time" | bc)
                
                # Parse test results
                local total_tests=$(echo "$test_output" | grep -c "=== RUN" || echo "0")
                local passed_tests=$(echo "$test_output" | grep -c "--- PASS:" || echo "0")
                local failed_tests=$(echo "$test_output" | grep -c "--- FAIL:" || echo "0")
                local skipped_tests=$(echo "$test_output" | grep -c "--- SKIP:" || echo "0")
                
                # Coverage analysis
                go test -coverprofile=coverage.out ./... > /dev/null 2>&1 || true
                local coverage_percent="0"
                if [ -f "coverage.out" ]; then
                    coverage_percent=$(go tool cover -func=coverage.out | tail -n 1 | awk '{print $3}' | sed 's/%//' || echo "0")
                    rm coverage.out
                fi
                
                test_data=$(echo "$test_data" | jq --arg component "$component" \
                    --arg duration "$test_duration" \
                    --arg total "$total_tests" \
                    --arg passed "$passed_tests" \
                    --arg failed "$failed_tests" \
                    --arg skipped "$skipped_tests" \
                    --arg coverage "$coverage_percent" \
                    '. + {($component): {
                        "duration_seconds": ($duration | tonumber),
                        "total_tests": ($total | tonumber),
                        "passed_tests": ($passed | tonumber),
                        "failed_tests": ($failed | tonumber),
                        "skipped_tests": ($skipped | tonumber),
                        "coverage_percent": ($coverage | tonumber),
                        "success_rate": (($passed | tonumber) / (($total | tonumber) + 0.1))
                    }}')
                
                cd ..
            fi
        done
    fi
    
    # Frontend tests
    if command -v npm &> /dev/null && [ -d "dashboard-v2" ]; then
        cd dashboard-v2
        
        local start_time=$(date +%s.%N)
        npm test -- --watchAll=false --coverage --silent > /dev/null 2>&1 || true
        local end_time=$(date +%s.%N)
        local frontend_test_duration=$(echo "$end_time - $start_time" | bc)
        
        # Extract coverage if available
        local frontend_coverage="0"
        if [ -f "coverage/lcov-report/index.html" ]; then
            frontend_coverage=$(grep -o '[0-9.]*%' coverage/lcov-report/index.html | head -n 1 | sed 's/%//' || echo "0")
        fi
        
        test_data=$(echo "$test_data" | jq --arg duration "$frontend_test_duration" \
            --arg coverage "$frontend_coverage" \
            '. + {
                "frontend": {
                    "duration_seconds": ($duration | tonumber),
                    "coverage_percent": ($coverage | tonumber)
                }
            }')
        
        cd ..
    fi
    
    # Update main report
    jq --argjson test_data "$test_data" '.test_performance = $test_data' "$REPORT_FILE" > /tmp/report.json && mv /tmp/report.json "$REPORT_FILE"
}

# Analyze cache effectiveness
analyze_cache_effectiveness() {
    print_status "Analyzing cache effectiveness..."
    
    local cache_data="{}"
    
    # Go cache analysis
    if command -v go &> /dev/null; then
        local go_cache_dir=$(go env GOCACHE)
        local go_cache_size=0
        local go_cache_files=0
        
        if [ -d "$go_cache_dir" ]; then
            go_cache_size=$(du -sb "$go_cache_dir" 2>/dev/null | cut -f1 || echo "0")
            go_cache_files=$(find "$go_cache_dir" -type f | wc -l || echo "0")
        fi
        
        local mod_cache_dir=$(go env GOMODCACHE)
        local mod_cache_size=0
        local mod_cache_modules=0
        
        if [ -d "$mod_cache_dir" ]; then
            mod_cache_size=$(du -sb "$mod_cache_dir" 2>/dev/null | cut -f1 || echo "0")
            mod_cache_modules=$(find "$mod_cache_dir" -maxdepth 2 -type d | wc -l || echo "0")
        fi
        
        cache_data=$(echo "$cache_data" | jq --arg go_cache_size "$go_cache_size" \
            --arg go_cache_files "$go_cache_files" \
            --arg mod_cache_size "$mod_cache_size" \
            --arg mod_cache_modules "$mod_cache_modules" \
            '. + {
                "go": {
                    "build_cache_size_bytes": ($go_cache_size | tonumber),
                    "build_cache_files": ($go_cache_files | tonumber),
                    "module_cache_size_bytes": ($mod_cache_size | tonumber),
                    "cached_modules": ($mod_cache_modules | tonumber)
                }
            }')
    fi
    
    # Node cache analysis
    if command -v npm &> /dev/null; then
        local npm_cache_dir=$(npm config get cache)
        local npm_cache_size=0
        local npm_cache_packages=0
        
        if [ -d "$npm_cache_dir" ]; then
            npm_cache_size=$(du -sb "$npm_cache_dir" 2>/dev/null | cut -f1 || echo "0")
            npm_cache_packages=$(find "$npm_cache_dir" -name "package.json" | wc -l || echo "0")
        fi
        
        cache_data=$(echo "$cache_data" | jq --arg npm_cache_size "$npm_cache_size" \
            --arg npm_cache_packages "$npm_cache_packages" \
            '. + {
                "npm": {
                    "cache_size_bytes": ($npm_cache_size | tonumber),
                    "cached_packages": ($npm_cache_packages | tonumber)
                }
            }')
    fi
    
    # Docker cache analysis
    if command -v docker &> /dev/null; then
        local docker_cache_info=$(docker system df --format "json" 2>/dev/null || echo '{}')
        
        cache_data=$(echo "$cache_data" | jq --argjson docker_info "$docker_cache_info" \
            '. + {"docker": $docker_info}')
    fi
    
    # Update main report
    jq --argjson cache_data "$cache_data" '.cache_analysis = $cache_data' "$REPORT_FILE" > /tmp/report.json && mv /tmp/report.json "$REPORT_FILE"
}

# Generate recommendations
generate_recommendations() {
    print_status "Generating optimization recommendations..."
    
    local recommendations='[]'
    
    # Analyze the report data and generate recommendations
    local go_build_data=$(jq '.build_performance.go' "$REPORT_FILE")
    local node_build_data=$(jq '.build_performance.node' "$REPORT_FILE")
    local docker_data=$(jq '.docker_performance' "$REPORT_FILE")
    local test_data=$(jq '.test_performance' "$REPORT_FILE")
    
    # Go-specific recommendations
    if [ "$go_build_data" != "null" ]; then
        # Check for slow builds
        local max_build_time=$(echo "$go_build_data" | jq '[.[].cold_build_seconds] | max // 0')
        if (( $(echo "$max_build_time > 30" | bc -l) )); then
            recommendations=$(echo "$recommendations" | jq '. + [{
                "category": "go_build",
                "priority": "high",
                "title": "Slow Go Build Detected",
                "description": "Go build times exceed 30 seconds. Consider using build caching and parallel compilation.",
                "actions": [
                    "Enable Go module proxy (GOPROXY=https://proxy.golang.org,direct)",
                    "Use go build -a flag for cleaner builds",
                    "Consider using go:embed for static assets",
                    "Profile build with go build -x for bottleneck analysis"
                ]
            }]')
        fi
        
        # Check cache effectiveness
        local min_cache_effectiveness=$(echo "$go_build_data" | jq '[.[].cache_effectiveness] | min // 0')
        if (( $(echo "$min_cache_effectiveness < 3" | bc -l) )); then
            recommendations=$(echo "$recommendations" | jq '. + [{
                "category": "go_cache",
                "priority": "medium",
                "title": "Go Cache Not Effective",
                "description": "Go build cache is not providing significant speedup. Cache may be corrupted or not properly utilized.",
                "actions": [
                    "Clean and rebuild cache with go clean -cache",
                    "Ensure GOCACHE is on fast storage (SSD)",
                    "Consider using shared cache in CI/CD",
                    "Verify cache directory permissions"
                ]
            }]')
        fi
    fi
    
    # Node.js-specific recommendations
    if [ "$node_build_data" != "null" ]; then
        local node_install_time=$(echo "$node_build_data" | jq '.install_seconds // 0')
        if (( $(echo "$node_install_time > 60" | bc -l) )); then
            recommendations=$(echo "$recommendations" | jq '. + [{
                "category": "node_build",
                "priority": "high",
                "title": "Slow npm install Detected",
                "description": "npm install takes more than 60 seconds. Consider optimization strategies.",
                "actions": [
                    "Use npm ci instead of npm install in CI/CD",
                    "Enable npm cache and ensure it is on fast storage",
                    "Consider using pnpm for faster installs",
                    "Audit and remove unused dependencies",
                    "Use .npmrc for registry optimization"
                ]
            }]')
        fi
        
        local bundle_size=$(echo "$node_build_data" | jq '.bundle_size_bytes // 0')
        if (( $(echo "$bundle_size > 5000000" | bc -l) )); then # > 5MB
            recommendations=$(echo "$recommendations" | jq '. + [{
                "category": "bundle_size",
                "priority": "medium",
                "title": "Large Bundle Size Detected",
                "description": "Frontend bundle exceeds 5MB. Consider code splitting and optimization.",
                "actions": [
                    "Implement code splitting with dynamic imports",
                    "Enable tree shaking in build configuration",
                    "Optimize images and use modern formats (WebP, AVIF)",
                    "Use bundle analyzer to identify large dependencies",
                    "Consider lazy loading for non-critical components"
                ]
            }]')
        fi
    fi
    
    # Docker-specific recommendations
    if [ "$docker_data" != "null" ]; then
        local docker_summary=$(echo "$docker_data" | jq '.summary // {}')
        local buildkit_enabled=$(echo "$docker_summary" | jq '.buildkit_enabled // false')
        
        if [ "$buildkit_enabled" != "true" ]; then
            recommendations=$(echo "$recommendations" | jq '. + [{
                "category": "docker_optimization",
                "priority": "high",
                "title": "Docker BuildKit Not Enabled",
                "description": "Docker BuildKit provides significant build performance improvements.",
                "actions": [
                    "Enable BuildKit with DOCKER_BUILDKIT=1",
                    "Use multi-stage builds with cache mounts",
                    "Implement cache mount syntax for package managers",
                    "Consider using Docker Compose build contexts"
                ]
            }]')
        fi
        
        # Check for slow Docker builds
        for component in controller agent dashboard; do
            local cold_build=$(echo "$docker_data" | jq --arg comp "$component" '.[$comp].cold_build_seconds // 0')
            if (( $(echo "$cold_build > 120" | bc -l) )); then # > 2 minutes
                recommendations=$(echo "$recommendations" | jq --arg comp "$component" '. + [{
                    "category": "docker_build",
                    "priority": "medium",
                    "title": ("Slow Docker Build for " + $comp),
                    "description": ("Docker build for " + $comp + " takes more than 2 minutes."),
                    "actions": [
                        "Optimize Dockerfile layer caching",
                        "Use smaller base images (alpine variants)",
                        "Minimize context size with .dockerignore",
                        "Use cache-from and cache-to build arguments"
                    ]
                }]')
            fi
        done
    fi
    
    # Test performance recommendations
    if [ "$test_data" != "null" ]; then
        for component in controller agent cli; do
            local test_duration=$(echo "$test_data" | jq --arg comp "$component" '.[$comp].duration_seconds // 0')
            if (( $(echo "$test_duration > 30" | bc -l) )); then
                recommendations=$(echo "$recommendations" | jq --arg comp "$component" '. + [{
                    "category": "test_performance",
                    "priority": "medium",
                    "title": ("Slow Tests in " + $comp),
                    "description": ("Test suite for " + $comp + " takes more than 30 seconds."),
                    "actions": [
                        "Use t.Parallel() for independent tests",
                        "Implement test caching with testify",
                        "Mock external dependencies",
                        "Use build tags to separate unit/integration tests",
                        "Consider running tests in parallel with -p flag"
                    ]
                }]')
            fi
            
            local coverage=$(echo "$test_data" | jq --arg comp "$component" '.[$comp].coverage_percent // 0')
            if (( $(echo "$coverage < 80" | bc -l) )); then
                recommendations=$(echo "$recommendations" | jq --arg comp "$component" '. + [{
                    "category": "test_coverage",
                    "priority": "low",
                    "title": ("Low Test Coverage in " + $comp),
                    "description": ("Test coverage for " + $comp + " is below 80%."),
                    "actions": [
                        "Add unit tests for uncovered functions",
                        "Implement table-driven tests for edge cases",
                        "Add integration tests for critical paths",
                        "Use coverage tools to identify gaps"
                    ]
                }]')
            fi
        done
    fi
    
    # General CI/CD recommendations
    recommendations=$(echo "$recommendations" | jq '. + [{
        "category": "ci_optimization",
        "priority": "medium",
        "title": "CI/CD Pipeline Optimization",
        "description": "General recommendations for improving CI/CD performance.",
        "actions": [
            "Use matrix builds for parallel execution",
            "Implement smart change detection to skip unnecessary jobs",
            "Use self-hosted runners for consistent performance",
            "Cache dependencies between builds",
            "Use fail-fast strategy for quick feedback"
        ]
    }]')
    
    # Update main report
    jq --argjson recommendations "$recommendations" '.recommendations = $recommendations' "$REPORT_FILE" > /tmp/report.json && mv /tmp/report.json "$REPORT_FILE"
}

# Generate HTML report
generate_html_report() {
    print_status "Generating HTML report..."
    
    cat > "$HTML_REPORT" << 'EOF'
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>ChaosLabs CI/CD Performance Report</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; margin: 0; padding: 20px; background: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; background: white; padding: 30px; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        h1, h2, h3 { color: #333; }
        .metric-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; margin: 20px 0; }
        .metric-card { padding: 20px; border: 1px solid #e0e0e0; border-radius: 6px; background: #fafafa; }
        .metric-value { font-size: 2em; font-weight: bold; color: #2196F3; }
        .metric-label { color: #666; font-size: 0.9em; }
        .recommendation { padding: 15px; margin: 10px 0; border-left: 4px solid #ff9800; background: #fff3e0; }
        .recommendation.high { border-left-color: #f44336; background: #ffebee; }
        .recommendation.medium { border-left-color: #ff9800; background: #fff3e0; }
        .recommendation.low { border-left-color: #4caf50; background: #e8f5e8; }
        .chart-container { margin: 20px 0; }
        .actions { margin-top: 10px; }
        .actions li { margin: 5px 0; }
        table { width: 100%; border-collapse: collapse; margin: 20px 0; }
        th, td { padding: 12px; text-align: left; border-bottom: 1px solid #ddd; }
        th { background-color: #f5f5f5; }
        .status-badge { padding: 4px 8px; border-radius: 4px; font-size: 0.8em; font-weight: bold; }
        .status-success { background: #e8f5e8; color: #4caf50; }
        .status-warning { background: #fff3e0; color: #ff9800; }
        .status-error { background: #ffebee; color: #f44336; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🚀 ChaosLabs CI/CD Performance Report</h1>
        <p>Generated: <span id="timestamp"></span></p>
        
        <div id="summary-section">
            <h2>📊 Performance Summary</h2>
            <div class="metric-grid" id="summary-metrics"></div>
        </div>
        
        <div id="charts-section">
            <h2>📈 Performance Charts</h2>
            <div class="chart-container">
                <canvas id="buildTimesChart" width="400" height="200"></canvas>
            </div>
            <div class="chart-container">
                <canvas id="cacheEffectivenessChart" width="400" height="200"></canvas>
            </div>
        </div>
        
        <div id="detailed-section">
            <h2>🔍 Detailed Analysis</h2>
            <div id="detailed-tables"></div>
        </div>
        
        <div id="recommendations-section">
            <h2>💡 Optimization Recommendations</h2>
            <div id="recommendations-list"></div>
        </div>
    </div>

    <script>
        // Load and display report data
        const reportData = REPORT_DATA_PLACEHOLDER;
        
        // Display timestamp
        document.getElementById('timestamp').textContent = new Date(reportData.timestamp).toLocaleString();
        
        // Generate summary metrics
        const summaryMetrics = document.getElementById('summary-metrics');
        
        // Build performance metrics
        if (reportData.build_performance.go) {
            const goData = reportData.build_performance.go;
            const avgBuildTime = Object.values(goData).filter(v => typeof v === 'object' && v.cold_build_seconds)
                .reduce((sum, v) => sum + v.cold_build_seconds, 0) / Object.keys(goData).length;
            
            summaryMetrics.innerHTML += `
                <div class="metric-card">
                    <div class="metric-value">${avgBuildTime.toFixed(1)}s</div>
                    <div class="metric-label">Average Go Build Time</div>
                </div>
            `;
        }
        
        // Test performance metrics
        if (reportData.test_performance) {
            const testData = reportData.test_performance;
            const totalTests = Object.values(testData).filter(v => typeof v === 'object' && v.total_tests)
                .reduce((sum, v) => sum + v.total_tests, 0);
            
            summaryMetrics.innerHTML += `
                <div class="metric-card">
                    <div class="metric-value">${totalTests}</div>
                    <div class="metric-label">Total Tests</div>
                </div>
            `;
        }
        
        // Cache metrics
        if (reportData.cache_analysis.go) {
            const cacheSize = (reportData.cache_analysis.go.build_cache_size_bytes / 1024 / 1024).toFixed(1);
            summaryMetrics.innerHTML += `
                <div class="metric-card">
                    <div class="metric-value">${cacheSize} MB</div>
                    <div class="metric-label">Go Build Cache Size</div>
                </div>
            `;
        }
        
        // Generate build times chart
        if (reportData.build_performance.go) {
            const ctx = document.getElementById('buildTimesChart').getContext('2d');
            const goData = reportData.build_performance.go;
            const components = Object.keys(goData).filter(k => typeof goData[k] === 'object' && goData[k].cold_build_seconds);
            
            new Chart(ctx, {
                type: 'bar',
                data: {
                    labels: components,
                    datasets: [{
                        label: 'Cold Build Time (s)',
                        data: components.map(c => goData[c].cold_build_seconds),
                        backgroundColor: 'rgba(255, 99, 132, 0.2)',
                        borderColor: 'rgba(255, 99, 132, 1)',
                        borderWidth: 1
                    }, {
                        label: 'Warm Build Time (s)',
                        data: components.map(c => goData[c].warm_build_seconds),
                        backgroundColor: 'rgba(54, 162, 235, 0.2)',
                        borderColor: 'rgba(54, 162, 235, 1)',
                        borderWidth: 1
                    }]
                },
                options: {
                    responsive: true,
                    scales: {
                        y: { beginAtZero: true }
                    },
                    plugins: {
                        title: {
                            display: true,
                            text: 'Build Times by Component'
                        }
                    }
                }
            });
        }
        
        // Generate recommendations
        const recommendationsList = document.getElementById('recommendations-list');
        if (reportData.recommendations) {
            reportData.recommendations.forEach(rec => {
                const actions = rec.actions.map(action => `<li>${action}</li>`).join('');
                recommendationsList.innerHTML += `
                    <div class="recommendation ${rec.priority}">
                        <h3>${rec.title}</h3>
                        <p>${rec.description}</p>
                        <div class="actions">
                            <strong>Recommended Actions:</strong>
                            <ul>${actions}</ul>
                        </div>
                    </div>
                `;
            });
        }
        
        // Generate detailed tables
        const detailedTables = document.getElementById('detailed-tables');
        
        // Go build details
        if (reportData.build_performance.go) {
            const goData = reportData.build_performance.go;
            const components = Object.keys(goData).filter(k => typeof goData[k] === 'object' && goData[k].cold_build_seconds);
            
            let tableHtml = `
                <h3>Go Build Performance</h3>
                <table>
                    <tr>
                        <th>Component</th>
                        <th>Cold Build (s)</th>
                        <th>Warm Build (s)</th>
                        <th>Cache Effectiveness</th>
                        <th>Test Build (s)</th>
                    </tr>
            `;
            
            components.forEach(comp => {
                const data = goData[comp];
                tableHtml += `
                    <tr>
                        <td>${comp}</td>
                        <td>${data.cold_build_seconds.toFixed(2)}</td>
                        <td>${data.warm_build_seconds.toFixed(2)}</td>
                        <td>${data.cache_effectiveness.toFixed(1)}x</td>
                        <td>${data.test_build_seconds.toFixed(2)}</td>
                    </tr>
                `;
            });
            
            tableHtml += '</table>';
            detailedTables.innerHTML += tableHtml;
        }
    </script>
</body>
</html>
EOF

    # Replace placeholder with actual data
    local json_data=$(cat "$REPORT_FILE" | jq -c .)
    sed -i "s/REPORT_DATA_PLACEHOLDER/$json_data/g" "$HTML_REPORT" 2>/dev/null || \
        sed -i '' "s/REPORT_DATA_PLACEHOLDER/$json_data/g" "$HTML_REPORT"
}

# Main execution
main() {
    print_status "Starting CI/CD performance analysis..."
    
    # Check dependencies
    if ! command -v jq &> /dev/null; then
        print_error "jq is required but not installed. Please install jq first."
        exit 1
    fi
    
    if ! command -v bc &> /dev/null; then
        print_error "bc is required but not installed. Please install bc first."
        exit 1
    fi
    
    # Initialize report
    init_report
    
    # Run analyses
    analyze_go_performance
    analyze_node_performance
    analyze_docker_performance
    analyze_test_performance
    analyze_cache_effectiveness
    generate_recommendations
    generate_html_report
    
    # Summary
    print_success "Performance analysis complete!"
    echo ""
    echo "📄 Reports generated:"
    echo "   JSON: $REPORT_FILE"
    echo "   HTML: $HTML_REPORT"
    echo ""
    
    # Quick summary
    if command -v jq &> /dev/null; then
        local total_recommendations=$(jq '.recommendations | length' "$REPORT_FILE")
        local high_priority=$(jq '[.recommendations[] | select(.priority == "high")] | length' "$REPORT_FILE")
        
        echo "📊 Quick Summary:"
        echo "   Total recommendations: $total_recommendations"
        echo "   High priority items: $high_priority"
        echo ""
        
        if [ "$high_priority" -gt 0 ]; then
            print_warning "⚠️  High priority optimizations available!"
            echo "Review the HTML report for detailed recommendations."
        else
            print_success "✅ No critical performance issues detected."
        fi
    fi
    
    echo ""
    echo "🔧 To apply optimizations, review the recommendations in the HTML report:"
    echo "   open $HTML_REPORT"
}

# Run main function
main "$@"