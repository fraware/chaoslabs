# HTTP Server Configuration Guide

This document describes the configuration options for the ChaosLabs controller's hardened HTTP server.

## Overview

The controller uses a robust HTTP server with the following security and performance features:

- **Timeouts**: Configurable read, write, and idle timeouts
- **Rate Limiting**: Per-client request rate limiting
- **Admission Control**: Bounded worker pool to prevent overload
- **Recovery**: Panic recovery middleware
- **Logging**: Request logging with timing information
- **CORS**: Cross-origin resource sharing support
- **Agent Request Handling**: Configurable timeouts, retries, and backoff for controller→agent calls

## Environment Variables

All configuration can be set via environment variables:

### Timeout Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `READ_HEADER_TIMEOUT` | `10s` | Maximum time to read request headers |
| `READ_TIMEOUT` | `30s` | Maximum time to read entire request body |
| `WRITE_TIMEOUT` | `30s` | Maximum time to write response |
| `IDLE_TIMEOUT` | `60s` | Maximum time to wait for next request on keep-alive connection |

### Resource Limits

| Variable | Default | Description |
|----------|---------|-------------|
| `MAX_HEADER_BYTES` | `1048576` (1MB) | Maximum size of request headers |
| `MAX_CONNS` | `1000` | Maximum concurrent connections |
| `MAX_REQUESTS_PER_SEC` | `1000` | Maximum requests per second per client |
| `WORKER_POOL_SIZE` | `100` | Maximum concurrent request handlers |

### Agent Request Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `AGENT_REQUEST_TIMEOUT` | `30s` | Timeout for agent requests |
| `AGENT_REQUEST_MAX_RETRIES` | `3` | Maximum retry attempts for failed agent requests |
| `AGENT_REQUEST_RETRY_DELAY` | `1s` | Initial delay before first retry |
| `AGENT_REQUEST_BACKOFF_MULTIPLIER` | `2.0` | Exponential backoff multiplier for retries |

## Configuration Examples

### Development Environment

```bash
export READ_TIMEOUT="5s"
export WRITE_TIMEOUT="5s"
export WORKER_POOL_SIZE="10"
export MAX_REQUESTS_PER_SEC="100"
export AGENT_REQUEST_TIMEOUT="10s"
export AGENT_REQUEST_MAX_RETRIES="2"
```

### Production Environment

```bash
export READ_TIMEOUT="60s"
export WRITE_TIMEOUT="60s"
export IDLE_TIMEOUT="120s"
export WORKER_POOL_SIZE="500"
export MAX_REQUESTS_PER_SEC="5000"
export AGENT_REQUEST_TIMEOUT="60s"
export AGENT_REQUEST_MAX_RETRIES="5"
export AGENT_REQUEST_RETRY_DELAY="2s"
```

### High-Load Environment

```bash
export READ_TIMEOUT="30s"
export WRITE_TIMEOUT="30s"
export WORKER_POOL_SIZE="1000"
export MAX_REQUESTS_PER_SEC="10000"
export MAX_CONNS="5000"
export AGENT_REQUEST_TIMEOUT="45s"
export AGENT_REQUEST_MAX_RETRIES="3"
export AGENT_REQUEST_BACKOFF_MULTIPLIER="1.5"
```

## Security Features

### Slowloris Protection

The server is protected against slowloris attacks through:

- **ReadHeaderTimeout**: Limits time to read headers
- **ReadTimeout**: Limits time to read request body
- **IdleTimeout**: Closes idle connections

### Rate Limiting

Per-client rate limiting prevents abuse:

- **Default Rate**: 1000 requests per second per client
- **Burst Allowance**: 100 requests (10% of rate)
- **Client Identification**: Based on IP address or X-Forwarded-For header

### Admission Control

Worker pool prevents server overload:

- **Pool Size**: Configurable via `WORKER_POOL_SIZE`
- **Rejection**: Returns 503 Service Unavailable when pool is full
- **Graceful Degradation**: Server remains responsive under load

### Agent Request Resilience

Robust handling of controller→agent communication:

- **Configurable Timeouts**: Per-request timeout limits
- **Exponential Backoff**: Intelligent retry delays
- **Status-Based Retry**: Retry on server errors, fail fast on client errors
- **Connection Pooling**: Efficient HTTP connection reuse
- **Metrics Collection**: Comprehensive monitoring of agent request performance

## Performance Tuning

### Timeout Guidelines

| Use Case | Read Timeout | Write Timeout | Idle Timeout |
|----------|--------------|---------------|--------------|
| API endpoints | 30s | 30s | 60s |
| File uploads | 300s | 300s | 120s |
| Long-running operations | 600s | 600s | 180s |

### Worker Pool Sizing

```
Worker Pool Size = (CPU Cores × 2) + (Expected Concurrent Users × 0.1)
```

Example:
- 8 CPU cores
- 1000 expected concurrent users
- Recommended: (8 × 2) + (1000 × 0.1) = 16 + 100 = 116 workers

### Rate Limiting Tuning

```
Requests per Second = (Expected QPS × Safety Factor) / Number of Load Balancers
```

Example:
- Expected QPS: 5000
- Safety factor: 1.2
- Load balancers: 2
- Recommended: (5000 × 1.2) / 2 = 3000 requests/sec

### Agent Request Tuning

```
Agent Request Timeout = (Expected Agent Response Time × 2) + Network Latency
Retry Delay = Network RTT × 1.5
Max Retries = log2(Desired Success Rate) / log2(Expected Failure Rate)
```

Example:
- Expected agent response: 10s
- Network latency: 100ms
- Recommended timeout: (10s × 2) + 100ms = 20.1s
- Recommended retry delay: 150ms
- For 99.9% success rate with 10% failure rate: log2(0.999) / log2(0.1) ≈ 3 retries

## Monitoring and Metrics

### Prometheus Metrics

The server exposes the following metrics:

- `http_requests_total`: Total HTTP requests
- `http_request_duration_seconds`: Request duration histogram
- `http_requests_in_flight`: Current requests being processed
- `rate_limit_exceeded_total`: Rate limit violations
- `controller_agent_requests_total`: Agent request outcomes (success, failed, client_error)
- `controller_agent_request_duration_seconds`: Agent request duration histogram
- `controller_agent_request_retries_total`: Agent request retry count

### Health Checks

```bash
# Check server health
curl -f http://localhost:8080/health

# Check metrics
curl http://localhost:8080/metrics

# Check readiness
curl -f http://localhost:8080/ready
```

## Troubleshooting

### Common Issues

#### High Latency

1. **Check resource usage**: CPU/memory bottlenecks
2. **Verify network**: Network interface saturation
3. **Review logs**: Error patterns or slow operations
4. **Check dependencies**: Database, external services
5. **Monitor agent requests**: Check retry patterns and timeouts

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
5. **Agent connectivity**: Verify agent endpoints and timeouts

#### Agent Request Failures

1. **Check agent health**: Verify agent is responding
2. **Review timeout settings**: Adjust `AGENT_REQUEST_TIMEOUT` if needed
3. **Monitor retry patterns**: Check if retries are helping or causing issues
4. **Network connectivity**: Verify network between controller and agents
5. **Agent capacity**: Check if agents are overloaded

### Debug Mode

Enable debug logging:

```bash
export DEBUG=1
export LOG_LEVEL=debug
```

### Performance Profiling

Generate CPU profile:

```bash
curl -o cpu.prof http://localhost:8080/debug/pprof/profile?seconds=30
go tool pprof cpu.prof
```

## Best Practices

### 1. Timeout Configuration

- Set `READ_TIMEOUT` based on expected request sizes
- Set `WRITE_TIMEOUT` based on response complexity
- Use `IDLE_TIMEOUT` to prevent connection exhaustion
- Configure `AGENT_REQUEST_TIMEOUT` based on agent performance characteristics

### 2. Resource Limits

- Monitor worker pool utilization
- Adjust pool size based on CPU and memory usage
- Set rate limits based on expected traffic patterns
- Balance retry settings with system capacity

### 3. Monitoring

- Track request latency percentiles (P50, P95, P99)
- Monitor error rates and types
- Alert on worker pool exhaustion
- Track agent request success rates and retry patterns

### 4. Security

- Use reverse proxy for additional protection
- Implement request validation
- Monitor for unusual traffic patterns
- Set appropriate retry limits to prevent abuse

## Configuration Validation

The server validates configuration on startup:

```bash
# Check configuration
curl -s http://localhost:8080/config | jq '.'

# Validate timeouts
curl -s http://localhost:8080/config | jq '.timeouts'

# Check agent request settings
curl -s http://localhost:8080/config | jq '.agent_requests'
```

## Migration Guide

### From Basic HTTP Server

1. Replace `http.ListenAndServe` with `server.Start`
2. Add graceful shutdown handling
3. Configure timeouts and limits
4. Add monitoring and metrics
5. Configure agent request resilience

### From Other Frameworks

1. Adapt middleware to Go's `http.Handler` interface
2. Configure timeouts appropriately
3. Implement graceful shutdown
4. Add health checks and metrics
5. Configure agent communication patterns

## References

- [Go HTTP Server Best Practices](https://golang.org/doc/gc)
- [HTTP Timeout Best Practices](https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/)
- [Rate Limiting Strategies](https://adam-p.ca/blog/2022/01/rate-limiting/)
- [Production Go](https://github.com/golang/go/wiki/Production)
- [Exponential Backoff](https://en.wikipedia.org/wiki/Exponential_backoff)
