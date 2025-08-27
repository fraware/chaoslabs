#!/bin/bash

# ChaosLabs Comprehensive Benchmarking Script
# This script runs performance benchmarks and generates reports for CI integration

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
CONTROLLER_URL="${CONTROLLER_URL:-http://localhost:8080}"
AGENT_URL="${AGENT_URL:-http://localhost:9090}"
BENCHMARK_DURATION="${BENCHMARK_DURATION:-5m}"
CONCURRENCY="${CONCURRENCY:-10}"
EXPERIMENT_TYPE="${EXPERIMENT_TYPE:-network-latency}"
OUTPUT_DIR="${OUTPUT_DIR:-./results}"
FLAMEGRAPH_DURATION="${FLAMEGRAPH_DURATION:-30}"

# Create output directory
mkdir -p "$OUTPUT_DIR"

echo -e "${BLUE}ChaosLabs Performance Benchmarking Suite${NC}"
echo "=============================================="
echo "Controller URL: $CONTROLLER_URL"
echo "Agent URL: $AGENT_URL"
echo "Duration: $BENCHMARK_DURATION"
echo "Concurrency: $CONCURRENCY"
echo "Experiment Type: $EXPERIMENT_TYPE"
echo "Output Directory: $OUTPUT_DIR"
echo ""

# Function to check service health
check_service_health() {
    local service_name=$1
    local url=$2
    local max_retries=30
    local retry_count=0
    
    echo -e "${YELLOW}Checking $service_name health at $url...${NC}"
    
    while [ $retry_count -lt $max_retries ]; do
        if curl -f -s "$url/health" > /dev/null 2>&1; then
            echo -e "${GREEN}✓ $service_name is healthy${NC}"
            return 0
        elif curl -f -s "$url/metrics" > /dev/null 2>&1; then
            echo -e "${GREEN}✓ $service_name is responding (metrics endpoint)${NC}"
            return 0
        fi
        
        echo -e "${YELLOW}Waiting for $service_name... (attempt $((retry_count + 1))/$max_retries)${NC}"
        sleep 2
        retry_count=$((retry_count + 1))
    done
    
    echo -e "${RED}✗ $service_name failed to become healthy${NC}"
    return 1
}

# Function to run benchmark
run_benchmark() {
    local config_name=$1
    local duration=$2
    local concurrency=$3
    local experiment_type=$4
    
    echo -e "${BLUE}Running $config_name benchmark...${NC}"
    echo "Duration: $duration, Concurrency: $concurrency, Type: $experiment_type"
    
    # Set environment variables for this benchmark
    export BENCHMARK_DURATION="$duration"
    export BENCHMARK_CONCURRENCY="$concurrency"
    export EXPERIMENT_TYPE="$experiment_type"
    
    # Run benchmark
    timeout "$duration" go run benchmark.go "$CONTROLLER_URL" "$AGENT_URL" 2>&1 | tee "$OUTPUT_DIR/${config_name}_benchmark.log"
    
    # Find and move benchmark results
    local benchmark_file=$(ls benchmark_*.json 2>/dev/null | head -1)
    if [ -n "$benchmark_file" ]; then
        mv "$benchmark_file" "$OUTPUT_DIR/${config_name}_results.json"
        echo -e "${GREEN}✓ $config_name benchmark completed${NC}"
    else
        echo -e "${RED}✗ $config_name benchmark failed - no results file found${NC}"
        return 1
    fi
}

# Function to generate flamegraphs
generate_flamegraphs() {
    echo -e "${BLUE}Generating flamegraphs...${NC}"
    
    if [ -f "./flamegraph.sh" ]; then
        chmod +x ./flamegraph.sh
        timeout "$FLAMEGRAPH_DURATION" ./flamegraph.sh -d "$FLAMEGRAPH_DURATION" 2>&1 | tee "$OUTPUT_DIR/flamegraph_generation.log"
        
        # Move generated flamegraphs
        for file in *_flamegraph.svg *_profile.pb.gz system_metrics.txt; do
            if [ -f "$file" ]; then
                mv "$file" "$OUTPUT_DIR/"
            fi
        done
        
        echo -e "${GREEN}✓ Flamegraphs generated${NC}"
    else
        echo -e "${YELLOW}Flamegraph script not found, skipping flamegraph generation${NC}"
    fi
}

# Function to collect system metrics
collect_system_metrics() {
    echo -e "${BLUE}Collecting system metrics...${NC}"
    
    # CPU info
    echo "=== CPU Information ===" > "$OUTPUT_DIR/system_info.txt"
    lscpu >> "$OUTPUT_DIR/system_info.txt" 2>/dev/null || echo "lscpu not available" >> "$OUTPUT_DIR/system_info.txt"
    
    # Memory info
    echo -e "\n=== Memory Information ===" >> "$OUTPUT_DIR/system_info.txt"
    free -h >> "$OUTPUT_DIR/system_info.txt" 2>/dev/null || echo "free not available" >> "$OUTPUT_DIR/system_info.txt"
    
    # Network interfaces
    echo -e "\n=== Network Interfaces ===" >> "$OUTPUT_DIR/system_info.txt"
    ip addr >> "$OUTPUT_DIR/system_info.txt" 2>/dev/null || ifconfig >> "$OUTPUT_DIR/system_info.txt" 2>/dev/null || echo "Network info not available" >> "$OUTPUT_DIR/system_info.txt"
    
    # Kernel version
    echo -e "\n=== Kernel Version ===" >> "$OUTPUT_DIR/system_info.txt"
    uname -a >> "$OUTPUT_DIR/system_info.txt" 2>/dev/null || echo "uname not available" >> "$OUTPUT_DIR/system_info.txt"
    
    echo -e "${GREEN}✓ System metrics collected${NC}"
}

# Function to generate performance report
generate_performance_report() {
    echo -e "${BLUE}Generating performance report...${NC}"
    
    local report_file="$OUTPUT_DIR/performance_report.md"
    
    cat > "$report_file" << EOF
# ChaosLabs Performance Benchmark Report

**Generated:** $(date -u +"%Y-%m-%d %H:%M UTC")
**Controller URL:** $CONTROLLER_URL
**Agent URL:** $AGENT_URL
**Benchmark Duration:** $BENCHMARK_DURATION
**Concurrency:** $CONCURRENCY

## Benchmark Results

EOF
    
    # Add results from each benchmark
    for results_file in "$OUTPUT_DIR"/*_results.json; do
        if [ -f "$results_file" ]; then
            local benchmark_name=$(basename "$results_file" _results.json)
            echo "### $benchmark_name" >> "$report_file"
            
            # Extract key metrics using jq if available
            if command -v jq > /dev/null 2>&1; then
                echo "**HTTP Latency:**" >> "$report_file"
                jq -r '.http_latency | "- P50: \(.p50)\n- P95: \(.p95)\n- P99: \(.p99)\n- Mean: \(.mean)\n- Count: \(.count)"' "$results_file" >> "$report_file" 2>/dev/null || echo "- Metrics extraction failed" >> "$report_file"
                
                echo "" >> "$report_file"
                echo "**Resource Usage:**" >> "$report_file"
                jq -r '.cpu_usage | "- CPU P95: \(.p95)%"' "$results_file" >> "$report_file" 2>/dev/null || echo "- CPU metrics extraction failed" >> "$report_file"
                jq -r '.memory_usage | "- Memory P95: \(.p95)%"' "$results_file" >> "$report_file" 2>/dev/null || echo "- Memory metrics extraction failed" >> "$report_file"
            else
                echo "- Install jq for detailed metrics extraction" >> "$report_file"
            fi
            
            echo "" >> "$report_file"
        fi
    done
    
    # Add generated files list
    echo "## Generated Files" >> "$report_file"
    echo "" >> "$report_file"
    ls -la "$OUTPUT_DIR"/*.json "$OUTPUT_DIR"/*.svg "$OUTPUT_DIR"/*.pb.gz "$OUTPUT_DIR"/*.txt 2>/dev/null | sed 's/^/- /' >> "$report_file" || echo "- No additional files found" >> "$report_file"
    
    echo -e "${GREEN}✓ Performance report generated: $report_file${NC}"
}

# Function to run soak test
run_soak_test() {
    echo -e "${BLUE}Running soak test...${NC}"
    
    # Run a longer benchmark to test stability
    run_benchmark "soak" "10m" "20" "cpu-stress"
}

# Main execution
main() {
    echo -e "${YELLOW}Starting ChaosLabs benchmarking suite...${NC}"
    
    # Check service health
    check_service_health "Controller" "$CONTROLLER_URL" || exit 1
    check_service_health "Agent" "$AGENT_URL" || exit 1
    
    # Collect system information
    collect_system_metrics
    
    # Run different benchmark scenarios
    echo -e "${BLUE}Running benchmark scenarios...${NC}"
    
    # Baseline benchmark
    run_benchmark "baseline" "$BENCHMARK_DURATION" "$CONCURRENCY" "$EXPERIMENT_TYPE"
    
    # High concurrency test
    run_benchmark "high_concurrency" "2m" "50" "network-latency"
    
    # CPU stress test
    run_benchmark "cpu_stress" "2m" "10" "cpu-stress"
    
    # Memory stress test
    run_benchmark "memory_stress" "2m" "10" "mem-stress"
    
    # Soak test (if not in CI)
    if [ "${CI:-false}" != "true" ]; then
        run_soak_test
    fi
    
    # Generate flamegraphs
    generate_flamegraphs
    
    # Generate performance report
    generate_performance_report
    
    echo -e "${GREEN}✓ All benchmarks completed successfully!${NC}"
    echo -e "${BLUE}Results saved to: $OUTPUT_DIR${NC}"
    
    # Print summary
    echo ""
    echo -e "${BLUE}=== Benchmark Summary ===${NC}"
    echo "Total benchmark runs: $(ls "$OUTPUT_DIR"/*_results.json 2>/dev/null | wc -l)"
    echo "Flamegraphs: $(ls "$OUTPUT_DIR"/*_flamegraph.svg 2>/dev/null | wc -l)"
    echo "System metrics: $(ls "$OUTPUT_DIR"/*.txt 2>/dev/null | wc -l)"
    echo "Performance report: $OUTPUT_DIR/performance_report.md"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --controller-url)
            CONTROLLER_URL="$2"
            shift 2
            ;;
        --agent-url)
            AGENT_URL="$2"
            shift 2
            ;;
        --duration)
            BENCHMARK_DURATION="$2"
            shift 2
            ;;
        --concurrency)
            CONCURRENCY="$2"
            shift 2
            ;;
        --experiment-type)
            EXPERIMENT_TYPE="$2"
            shift 2
            ;;
        --output-dir)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        --flamegraph-duration)
            FLAMEGRAPH_DURATION="$2"
            shift 2
            ;;
        --help)
            echo "Usage: $0 [OPTIONS]"
            echo "Options:"
            echo "  --controller-url URL     Controller URL (default: http://localhost:8080)"
            echo "  --agent-url URL          Agent URL (default: http://localhost:9090)"
            echo "  --duration DURATION      Benchmark duration (default: 5m)"
            echo "  --concurrency N          Concurrency level (default: 10)"
            echo "  --experiment-type TYPE   Experiment type (default: network-latency)"
            echo "  --output-dir DIR         Output directory (default: ./results)"
            echo "  --flamegraph-duration N  Flamegraph duration in seconds (default: 30)"
            echo "  --help                   Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Run main function
main "$@"
