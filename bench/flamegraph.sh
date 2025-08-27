#!/bin/bash

# Flamegraph generation script for ChaosLabs
# Requires: go-torch, pprof

set -e

CONTROLLER_PID=""
AGENT_PID=""
DURATION=30

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}ChaosLabs Flamegraph Generator${NC}"
echo "================================"

# Function to find process PIDs
find_pids() {
    echo -e "${YELLOW}Finding process PIDs...${NC}"
    
    CONTROLLER_PID=$(pgrep -f "chaos-controller" || echo "")
    AGENT_PID=$(pgrep -f "chaos-agent" || echo "")
    
    if [ -z "$CONTROLLER_PID" ]; then
        echo -e "${RED}Controller process not found. Make sure it's running.${NC}"
        exit 1
    fi
    
    if [ -z "$AGENT_PID" ]; then
        echo -e "${RED}Agent process not found. Make sure it's running.${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}Controller PID: $CONTROLLER_PID${NC}"
    echo -e "${GREEN}Agent PID: $AGENT_PID${NC}"
}

# Function to generate CPU flamegraph
generate_cpu_flamegraph() {
    local pid=$1
    local name=$2
    local port=$3
    
    echo -e "${YELLOW}Generating CPU flamegraph for $name (PID: $pid)...${NC}"
    
    # Generate flamegraph using go-torch
    if command -v go-torch &> /dev/null; then
        go-torch -u http://localhost:$port/debug/pprof/profile -d $DURATION -f "${name}_cpu_flamegraph.svg"
        echo -e "${GREEN}CPU flamegraph saved as ${name}_cpu_flamegraph.svg${NC}"
    else
        echo -e "${YELLOW}go-torch not found, using pprof directly...${NC}"
        go tool pprof -proto http://localhost:$port/debug/pprof/profile > "${name}_cpu_profile.pb.gz"
        echo -e "${GREEN}CPU profile saved as ${name}_cpu_profile.pb.gz${NC}"
    fi
}

# Function to generate memory flamegraph
generate_memory_flamegraph() {
    local pid=$1
    local name=$2
    local port=$3
    
    echo -e "${YELLOW}Generating memory flamegraph for $name (PID: $pid)...${NC}"
    
    if command -v go-torch &> /dev/null; then
        go-torch -u http://localhost:$port/debug/pprof/heap -d $DURATION -f "${name}_memory_flamegraph.svg"
        echo -e "${GREEN}Memory flamegraph saved as ${name}_memory_flamegraph.svg${NC}"
    else
        echo -e "${YELLOW}go-torch not found, using pprof directly...${NC}"
        go tool pprof -proto http://localhost:$port/debug/pprof/heap > "${name}_memory_profile.pb.gz"
        echo -e "${GREEN}Memory profile saved as ${name}_memory_profile.pb.gz${NC}"
    fi
}

# Function to generate goroutine flamegraph
generate_goroutine_flamegraph() {
    local pid=$1
    local name=$2
    local port=$3
    
    echo -e "${YELLOW}Generating goroutine flamegraph for $name (PID: $pid)...${NC}"
    
    if command -v go-torch &> /dev/null; then
        go-torch -u http://localhost:$port/debug/pprof/goroutine -d $DURATION -f "${name}_goroutine_flamegraph.svg"
        echo -e "${GREEN}Goroutine flamegraph saved as ${name}_goroutine_flamegraph.svg${NC}"
    else
        echo -e "${YELLOW}go-torch not found, using pprof directly...${NC}"
        go tool pprof -proto http://localhost:$port/debug/pprof/goroutine > "${name}_goroutine_profile.pb.gz"
        echo -e "${GREEN}Goroutine profile saved as ${name}_goroutine_profile.pb.gz${NC}"
    fi
}

# Function to collect system metrics
collect_system_metrics() {
    echo -e "${YELLOW}Collecting system metrics...${NC}"
    
    # CPU usage
    echo "=== CPU Usage ===" > system_metrics.txt
    top -b -n 1 | head -20 >> system_metrics.txt
    
    # Memory usage
    echo -e "\n=== Memory Usage ===" >> system_metrics.txt
    free -h >> system_metrics.txt
    
    # Network statistics
    echo -e "\n=== Network Statistics ===" >> system_metrics.txt
    ss -s >> system_metrics.txt
    
    # Process tree
    echo -e "\n=== Process Tree ===" >> system_metrics.txt
    pstree -p >> system_metrics.txt
    
    echo -e "${GREEN}System metrics saved to system_metrics.txt${NC}"
}

# Main execution
main() {
    find_pids
    
    echo -e "${YELLOW}Starting flamegraph generation (duration: ${DURATION}s)...${NC}"
    
    # Generate flamegraphs for controller
    generate_cpu_flamegraph "$CONTROLLER_PID" "controller" "8080"
    generate_memory_flamegraph "$CONTROLLER_PID" "controller" "8080"
    generate_goroutine_flamegraph "$CONTROLLER_PID" "controller" "8080"
    
    # Generate flamegraphs for agent
    generate_cpu_flamegraph "$AGENT_PID" "agent" "9090"
    generate_memory_flamegraph "$AGENT_PID" "agent" "9090"
    generate_goroutine_flamegraph "$AGENT_PID" "agent" "9090"
    
    # Collect system metrics
    collect_system_metrics
    
    echo -e "${GREEN}Flamegraph generation completed!${NC}"
    echo -e "${YELLOW}Files generated:${NC}"
    ls -la *_flamegraph.svg *_profile.pb.gz system_metrics.txt 2>/dev/null || echo "No files found"
}

# Check dependencies
check_dependencies() {
    echo -e "${YELLOW}Checking dependencies...${NC}"
    
    if ! command -v go &> /dev/null; then
        echo -e "${RED}Go is not installed${NC}"
        exit 1
    fi
    
    if ! command -v pstree &> /dev/null; then
        echo -e "${YELLOW}pstree not found, some metrics may be limited${NC}"
    fi
    
    echo -e "${GREEN}Dependencies check passed${NC}"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -d|--duration)
            DURATION="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [-d|--duration SECONDS]"
            echo "  -d, --duration SECONDS  Duration for profiling (default: 30)"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

check_dependencies
main
