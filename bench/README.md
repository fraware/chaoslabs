# ChaosLabs Performance & Reliability Benchmarking

This directory contains the benchmarking framework for measuring the performance and reliability of the ChaosLabs controller→agent→kernel toolchain.

## Overview

The benchmarking framework measures:
- **HTTP Latency**: P50, P95, P99 request latencies between controller and agent
- **Resource Usage**: CPU and memory consumption during experiments
- **Kernel Metrics**: TCP retransmits, drops, interrupts during fault injection
- **Flamegraphs**: CPU, memory, and goroutine profiles for performance analysis

## Prerequisites

### Required Tools
- Go 1.21+
- `go-torch` for flamegraph generation (optional, falls back to pprof)
- `stress-ng` for system stress testing
- `tc` (traffic control) for network fault injection

### Installation
```bash
# Install go-torch for flamegraphs
go install github.com/uber/go-torch@latest

# Install stress-ng
# Ubuntu/Debian
sudo apt-get install stress-ng

# CentOS/RHEL
sudo yum install stress-ng

# macOS
brew install stress-ng
```

## Quick Start

### 1. Start Services
```bash
# Start controller, agent, and dashboard
docker-compose up -d

# Or run individually
cd controller && go run main.go &
cd agent && go run main.go &
cd Dashboard && python app.py &
```

### 2. Run Basic Benchmark
```bash
cd bench
go run benchmark.go
```

### 3. Generate Flamegraphs
```bash
chmod +x flamegraph.sh
./flamegraph.sh -d 60  # 60 second profiling
```

## Benchmark Configuration

### Environment Variables
```bash
export CONTROLLER_URL="http://localhost:8080"
export AGENT_URL="http://localhost:9090"
export BENCHMARK_DURATION="5m"
export BENCHMARK_CONCURRENCY="10"
```

### Command Line Options
```bash
go run benchmark.go [controller_url] [agent_url]
```

## Benchmark Types

### 1. HTTP Load Testing
- **Endpoint**: `/start` (experiment creation)
- **Metrics**: Request latency, throughput, error rates
- **Duration**: Configurable (default: 5 minutes)

### 2. Resource Monitoring
- **CPU Usage**: Per-process CPU consumption
- **Memory Usage**: RSS and heap usage
- **Goroutines**: Active goroutine count

### 3. Kernel Metrics
- **Network**: TCP retransmits, drops, congestion
- **System**: CPU interrupts, context switches
- **Storage**: I/O wait, disk utilization

## Output Files

### Benchmark Results
- `benchmark_YYYYMMDD_HHMMSS.json`: Complete benchmark results
- `system_metrics.txt`: System-level metrics snapshot

### Flamegraphs (if go-torch available)
- `controller_cpu_flamegraph.svg`: Controller CPU profile
- `controller_memory_flamegraph.svg`: Controller memory profile
- `controller_goroutine_flamegraph.svg`: Controller goroutine profile
- `agent_cpu_flamegraph.svg`: Agent CPU profile
- `agent_memory_flamegraph.svg`: Agent memory profile
- `agent_goroutine_flamegraph.svg`: Agent goroutine profile

### Profiles (fallback)
- `*_profile.pb.gz`: Go pprof profiles for offline analysis

## Interpreting Results

### Latency Percentiles
- **P50**: Median response time (50% of requests)
- **P95**: 95th percentile (95% of requests)
- **P99**: 99th percentile (99% of requests)

### Resource Usage
- **CPU**: Percentage of CPU cores used
- **Memory**: RSS (Resident Set Size) in MB
- **Goroutines**: Active goroutine count

### Kernel Metrics
- **TCP Retransmits**: Network packet loss/reordering
- **CPU Interrupts**: System load and I/O activity

## Performance Baselines

### Target Metrics (for reference)
```
HTTP Latency:
  P50:  < 10ms
  P95:  < 50ms
  P99:  < 100ms

Resource Usage:
  CPU:   < 20% per process
  Memory: < 100MB per process

Kernel Metrics:
  TCP Retransmits: < 1% of packets
  CPU Interrupts:  < 1000/sec
```

## Advanced Usage

### Custom Experiment Types
```go
// Modify benchmark.go to test different scenarios
expReq := ExperimentRequest{
    ExperimentType: "network-loss",
    Duration:       60,
    LossPercent:    10,
    // ... other parameters
}
```

### Long-Running Tests
```bash
# Run 2-hour soak test
go run benchmark.go &
sleep 2h
pkill -f "go run benchmark.go"
```

### Load Testing Scenarios
```bash
# Low concurrency
export BENCHMARK_CONCURRENCY="5"
go run benchmark.go

# High concurrency
export BENCHMARK_CONCURRENCY="100"
go run benchmark.go
```

## Troubleshooting

### Common Issues

#### Process Not Found
```bash
# Check if services are running
ps aux | grep -E "(controller|agent)"

# Check ports
netstat -tlnp | grep -E "(8080|9090)"
```

#### Permission Denied
```bash
# For kernel metrics collection
sudo chmod +r /proc/net/netstat /proc/stat

# For flamegraph generation
sudo setcap cap_sys_ptrace+ep $(which go-torch)
```

#### High Resource Usage
```bash
# Monitor in real-time
htop
iotop
nethogs
```

### Debug Mode
```bash
# Enable debug logging
export DEBUG=1
go run benchmark.go
```

## CI/CD Integration

### GitHub Actions
```yaml
- name: Run Benchmarks
  run: |
    cd bench
    go run benchmark.go
    ./flamegraph.sh -d 30
```

### Metrics Collection
```bash
# Parse results for CI
jq '.http_latency.p99' benchmark_*.json
jq '.cpu_usage.p95' benchmark_*.json
```

## Contributing

### Adding New Metrics
1. Extend the `BenchmarkResult` struct
2. Implement collection logic
3. Add to the main benchmark loop
4. Update documentation

### New Benchmark Types
1. Create new experiment configurations
2. Add specific measurement logic
3. Include in result aggregation
4. Update flamegraph generation

## References

- [Go Performance](https://golang.org/doc/gc)
- [pprof Documentation](https://golang.org/pkg/net/http/pprof/)
- [go-torch](https://github.com/uber/go-torch)
- [Linux Performance](https://brendangregg.com/linuxperf.html)
- [Network Performance](https://netdevconf.info/)
