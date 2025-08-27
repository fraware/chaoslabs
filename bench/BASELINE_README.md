# ChaosLabs Performance & Reliability Baseline

This document establishes the performance and reliability baseline for the ChaosLabs controller→agent→kernel toolchain. It defines performance targets, measurement methodologies, and provides guidance for interpreting benchmark results.

## Overview

The ChaosLabs benchmarking framework measures the end-to-end performance of the chaos engineering toolchain:

- **Controller**: HTTP API performance, scheduling overhead, and resource usage
- **Agent**: Fault injection execution time, resource consumption, and kernel interaction efficiency
- **Kernel**: Network impairment accuracy, system resource impact, and fault injection precision

## Performance Targets

### Latency Targets

| Metric | Target | Acceptable Range | Alert Threshold |
|--------|--------|------------------|-----------------|
| **P50 Latency** | < 10ms | 5-15ms | > 20ms |
| **P95 Latency** | < 50ms | 20-80ms | > 100ms |
| **P99 Latency** | < 100ms | 50-150ms | > 200ms |
| **Mean Latency** | < 25ms | 15-40ms | > 60ms |

### Resource Usage Targets

| Component | CPU (P95) | Memory (P95) | Network I/O |
|-----------|------------|---------------|-------------|
| **Controller** | < 15% | < 100MB | < 1MB/s |
| **Agent** | < 20% | < 150MB | < 2MB/s |
| **Combined** | < 30% | < 200MB | < 3MB/s |

### Kernel Metrics Targets

| Metric | Target | Alert Threshold |
|--------|--------|-----------------|
| **TCP Retransmits** | < 1% of packets | > 5% of packets |
| **CPU Interrupts** | < 1000/sec | > 5000/sec |
| **Network Drops** | < 0.1% | > 1% |

## Benchmark Scenarios

### 1. Baseline Performance
- **Duration**: 5 minutes
- **Concurrency**: 10 concurrent requests
- **Experiment Type**: Network latency (100ms)
- **Purpose**: Establish normal operating performance

### 2. High Concurrency Test
- **Duration**: 2 minutes
- **Concurrency**: 50 concurrent requests
- **Experiment Type**: Network latency (50ms)
- **Purpose**: Test system behavior under load

### 3. CPU Stress Test
- **Duration**: 2 minutes
- **Concurrency**: 10 concurrent requests
- **Experiment Type**: CPU stress (2 workers)
- **Purpose**: Measure CPU-bound experiment overhead

### 4. Memory Stress Test
- **Duration**: 2 minutes
- **Concurrency**: 10 concurrent requests
- **Experiment Type**: Memory stress (100MB)
- **Purpose**: Measure memory-bound experiment overhead

### 5. Soak Test
- **Duration**: 10 minutes
- **Concurrency**: 20 concurrent requests
- **Experiment Type**: CPU stress (1 worker)
- **Purpose**: Test long-term stability and resource leaks

## Measurement Methodology

### HTTP Latency Measurement
- **Tool**: Custom Go benchmark client
- **Method**: Round-trip time for experiment creation requests
- **Sampling**: Continuous measurement during benchmark duration
- **Calculation**: Proper percentile calculation with sorted data

### Resource Usage Measurement
- **Tool**: `ps` command with custom parsing
- **Metrics**: CPU percentage, memory RSS
- **Sampling**: 1-second intervals
- **Calculation**: Statistical aggregation (min, max, mean, P95, P99)

### Kernel Metrics Collection
- **Tool**: Direct `/proc` filesystem access
- **Metrics**: TCP statistics, CPU interrupts, network drops
- **Sampling**: End-of-benchmark snapshot
- **Calculation**: Raw values and derived percentages

### Flamegraph Generation
- **Tool**: `go-torch` with fallback to `pprof`
- **Profiles**: CPU, memory, goroutine
- **Duration**: 30 seconds per profile
- **Output**: SVG flamegraphs and pprof profiles

## Running Benchmarks

### Local Development
```bash
cd bench

# Run basic benchmark
go run benchmark.go

# Run comprehensive suite
./run_benchmarks.sh

# Run with custom parameters
./run_benchmarks.sh \
  --duration "10m" \
  --concurrency "50" \
  --experiment-type "cpu-stress"
```

### CI/CD Integration
```bash
# Set CI environment
export CI=true
export BENCHMARK_DURATION="3m"
export CONCURRENCY="20"

# Run benchmarks
./run_benchmarks.sh \
  --output-dir "./ci_results"
```

### Docker Environment
```bash
# Start services
docker-compose up -d

# Run benchmarks
./run_benchmarks.sh \
  --controller-url "http://localhost:8080" \
  --agent-url "http://localhost:9090"
```

## Interpreting Results

### Performance Analysis

#### Latency Patterns
- **Normal**: P99 < 3x P50, consistent distribution
- **Concerning**: P99 > 5x P50, long tail distribution
- **Critical**: P99 > 10x P50, bimodal distribution

#### Resource Usage Patterns
- **Efficient**: CPU < 10%, memory stable
- **Concerning**: CPU > 20%, memory growing
- **Critical**: CPU > 50%, memory leak pattern

#### Kernel Impact
- **Minimal**: < 0.1% packet loss, < 100 interrupts/sec
- **Concerning**: 0.1-1% packet loss, 100-1000 interrupts/sec
- **Critical**: > 1% packet loss, > 1000 interrupts/sec

### Regression Detection

#### Performance Regression
- P99 latency increased by > 20%
- CPU usage increased by > 30%
- Memory usage increased by > 50%

#### Stability Issues
- Benchmark failures > 5%
- Resource usage variance > 100%
- Kernel metric anomalies

### Baseline Comparison

Compare current results against:
- **Previous commit**: Detect regressions
- **Release baseline**: Track long-term trends
- **Performance targets**: Identify gaps

## Troubleshooting

### Common Issues

#### High Latency
1. **Check resource usage**: CPU/memory bottlenecks
2. **Verify network**: Network interface saturation
3. **Review logs**: Error patterns or slow operations
4. **Check dependencies**: Database, external services

#### Resource Exhaustion
1. **Monitor memory**: Check for leaks
2. **CPU profiling**: Identify hot paths
3. **Goroutine count**: Check for goroutine leaks
4. **File descriptors**: Check ulimit settings

#### Benchmark Failures
1. **Service health**: Verify controller/agent status
2. **Dependencies**: Check required tools (tc, stress-ng)
3. **Permissions**: Verify required capabilities
4. **Network**: Check connectivity and firewall rules

### Debug Mode
```bash
# Enable verbose logging
export DEBUG=1
export LOG_LEVEL=debug

# Run with additional metrics
./run_benchmarks.sh --flamegraph-duration 60
```

## CI/CD Integration

### GitHub Actions
The benchmarking workflow runs:
- **On push to main**: Full benchmark suite
- **Daily at 2 AM UTC**: Trend analysis
- **Manual trigger**: Custom benchmark runs

### Artifacts
- **Benchmark results**: JSON files with metrics
- **Flamegraphs**: SVG performance profiles
- **Performance reports**: Markdown summaries
- **System metrics**: Hardware and OS information

### Performance Gates
- **Latency**: P99 < 100ms
- **Resource**: CPU < 20%, Memory < 200MB
- **Stability**: < 5% failure rate

## Extending the Framework

### Adding New Metrics
1. **Extend structures**: Add fields to `BenchmarkResult`
2. **Implement collection**: Add measurement logic
3. **Update calculation**: Add statistical analysis
4. **Document targets**: Define performance expectations

### New Benchmark Types
1. **Define scenario**: Duration, concurrency, experiment type
2. **Implement runner**: Add to benchmark suite
3. **Set targets**: Define performance expectations
4. **Add validation**: Ensure result quality

### Custom Experiments
1. **Extend agent**: Add new fault injection types
2. **Update controller**: Add experiment scheduling
3. **Benchmark integration**: Add to measurement suite
4. **Documentation**: Update this baseline

## References

### Performance Engineering
- [Go Performance](https://golang.org/doc/gc)
- [Linux Performance](https://brendangregg.com/linuxperf.html)
- [Network Performance](https://netdevconf.info/)

### Benchmarking Tools
- [go-torch](https://github.com/uber/go-torch)
- [pprof](https://golang.org/pkg/net/http/pprof/)
- [stress-ng](https://manpages.ubuntu.com/manpages/focal/man1/stress-ng.1.html)

### Chaos Engineering
- [Chaos Mesh](https://chaos-mesh.org/)
- [Litmus](https://litmuschaos.io/)
- [Gremlin](https://www.gremlin.com/)

## Maintenance

### Regular Tasks
- **Weekly**: Review performance trends
- **Monthly**: Update performance targets
- **Quarterly**: Review and update benchmark scenarios
- **Annually**: Comprehensive baseline review

### Performance Reviews
- **Regression analysis**: Identify performance changes
- **Target adjustment**: Update based on hardware improvements
- **Scenario evolution**: Add new test cases
- **Tool updates**: Keep benchmarking tools current

---

*This baseline document should be updated whenever performance targets change or new benchmark scenarios are added.*
