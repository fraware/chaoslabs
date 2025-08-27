#!/bin/bash

# ChaosLabs Agent Preflight System Demo
# This script demonstrates various ways to use the preflight system

set -e

echo "ChaosLabs Agent Preflight System Demo"
echo "=========================================="
echo

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    local status=$1
    local message=$2
    
    case $status in
        "success")
            echo -e "${GREEN}✅ $message${NC}"
            ;;
        "warning")
            echo -e "${YELLOW}⚠️  $message${NC}"
            ;;
        "error")
            echo -e "${RED}❌ $message${NC}"
            ;;
        "info")
            echo -e "${BLUE}ℹ️  $message${NC}"
            ;;
    esac
}

# Check if we're in the right directory
if [ ! -f "preflight.go" ]; then
    print_status "error" "Please run this script from the agent directory"
    exit 1
fi

print_status "info" "Building preflight system..."

# Build the agent
if make build > /dev/null 2>&1; then
    print_status "success" "Agent built successfully"
else
    print_status "error" "Failed to build agent"
    exit 1
fi

# Build the doctor CLI tool
if make doctor > /dev/null 2>&1; then
    print_status "success" "Doctor CLI built successfully"
else
    print_status "error" "Failed to build doctor CLI"
    exit 1
fi

echo
print_status "info" "Running preflight checks..."

# Run preflight checks
if [ -f "bin/chaoslabs-doctor" ]; then
    print_status "success" "Using chaoslabs doctor CLI"
    ./bin/chaoslabs-doctor
else
    print_status "warning" "Doctor CLI not found, using agent directly"
    ./bin/chaoslabs-agent --preflight
fi

echo
print_status "info" "Testing different output formats..."

# Test JSON output
if [ -f "bin/chaoslabs-doctor" ]; then
    print_status "info" "JSON output:"
    ./bin/chaoslabs-doctor --format json | head -20
    echo "..."
    
    print_status "info" "Saving results to file..."
    ./bin/chaoslabs-doctor --format json-pretty --output preflight_results.json
    if [ -f "preflight_results.json" ]; then
        print_status "success" "Results saved to preflight_results.json"
    else
        print_status "error" "Failed to save results"
    fi
fi

echo
print_status "info" "Running tests..."

# Run preflight tests
if make test-preflight > /dev/null 2>&1; then
    print_status "success" "Preflight tests passed"
else
    print_status "warning" "Some preflight tests failed (this may be expected in some environments)"
fi

echo
print_status "info" "Performance benchmarks..."

# Run benchmark tests
if make test-benchmark > /dev/null 2>&1; then
    print_status "success" "Benchmark tests completed"
else
    print_status "warning" "Benchmark tests failed (this may be expected in some environments)"
fi

echo
print_status "info" "Integration examples..."

# Show how to integrate preflight in Go code
cat << 'EOF'

📝 Integration Example (Go):

```go
import "fraware/chaos-controller"

// Before running any experiment
preflightManager := NewPreflightManager()
result, err := preflightManager.RunAllChecks()
if err != nil {
    return fmt.Errorf("preflight failed: %w", err)
}

if result.Summary.OverallStatus == CheckStatusFail {
    return fmt.Errorf("critical checks failed")
}

// Continue with experiment...
```

EOF

# Show how to integrate with health checks
cat << 'EOF'

🏥 Health Check Integration:

```go
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
    preflightManager := NewPreflightManager()
    result, err := preflightManager.RunAllChecks()
    
    if err != nil || result.Summary.OverallStatus == CheckStatusFail {
        http.Error(w, "Environment unhealthy", http.StatusServiceUnavailable)
        return
    }
    
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(result)
}
```

EOF

echo
print_status "info" "Container deployment examples..."

# Show Docker and Kubernetes examples
cat << 'EOF'

🐳 Docker Example:

```bash
docker run --cap-add=NET_ADMIN --cap-add=SYS_ADMIN \
  -v /proc:/proc -v /sys:/sys -v /dev:/dev \
  chaoslabs-agent:latest
```

☸️ Kubernetes Example:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: chaoslabs-agent
spec:
  template:
    spec:
      containers:
      - name: agent
        image: chaoslabs-agent:latest
        securityContext:
          capabilities:
            add:
            - NET_ADMIN
            - SYS_ADMIN
        volumeMounts:
        - name: proc
          mountPath: /proc
        - name: sys
          mountPath: /sys
        - name: dev
          mountPath: /dev
```

EOF

echo
print_status "info" "Available make targets:"

# Show available make targets
cat << 'EOF'

🔧 Make Targets:
- make build          - Build the agent
- make test           - Run all tests
- make test-preflight - Run preflight tests only
- make doctor         - Build and run doctor CLI
- make preflight      - Run preflight checks
- make clean          - Clean build artifacts
- make help           - Show all available targets

EOF

echo
print_status "info" "Troubleshooting common issues..."

# Show troubleshooting tips
cat << 'EOF'

🚨 Common Issues & Solutions:

1. "tc: command not found"
   Solution: sudo apt-get install iproute2

2. "Operation not permitted" with tc
   Solution: sudo setcap cap_net_admin+ep /path/to/agent

3. "Module not found" errors
   Solution: sudo modprobe sch_netem ifb

4. Container permission issues
   Solution: Run with --cap-add=NET_ADMIN

EOF

echo
print_status "success" "Demo completed successfully!"
print_status "info" "For more information, see:"
echo "  - docs/AGENT_PREFLIGHT_CHECKS.md (comprehensive documentation)"
echo "  - agent/README_PREFLIGHT.md (quick start guide)"
echo "  - agent/examples/ (more examples)"

echo
print_status "info" "Next steps:"
echo "  1. Review the preflight results above"
echo "  2. Fix any failed checks using the provided remediation steps"
echo "  3. Integrate preflight checks into your experiment workflow"
echo "  4. Set up monitoring and alerting for preflight failures"

echo
echo "🎉 Happy chaos engineering!"
