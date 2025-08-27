# ChaosLabs Agent Preflight System

The preflight system ensures your ChaosLabs agent is properly configured and has the necessary capabilities before running chaos engineering experiments.

## 🚀 Quick Start

### Run Preflight Checks

```bash
# Build and run the agent with preflight checks
make preflight

# Or use the dedicated doctor CLI tool
make doctor

# Run checks manually
cd cmd/doctor
go build -o chaoslabs-doctor .
./chaoslabs-doctor
```

### What Gets Checked

✅ **Capabilities**: CAP_NET_ADMIN, CAP_SYS_ADMIN, CAP_SYS_RESOURCE  
✅ **Tools**: tc, ip, stress-ng, cgcreate, ifconfig  
✅ **Container**: Privileges, mounts, environment  
✅ **System**: Kernel modules, memory, disk, CPU  
✅ **Network**: IFB/netem modules, interfaces  

## 🔧 Installation

### Prerequisites

```bash
# Install required packages
sudo apt-get install iproute2 stress-ng cgroup-tools net-tools

# Or on CentOS/RHEL
sudo yum install iproute-tc stress-ng libcgroup-tools net-tools
```

### Grant Capabilities

```bash
# Grant network capabilities
sudo setcap cap_net_admin+ep /path/to/chaoslabs-agent

# Grant system capabilities (optional)
sudo setcap cap_sys_admin+ep /path/to/chaoslabs-agent
sudo setcap cap_sys_resource+ep /path/to/chaoslabs-agent
```

### Load Kernel Modules

```bash
# Load required network modules
sudo modprobe sch_netem
sudo modprobe ifb
```

## 📋 Usage Examples

### Basic Preflight Check

```bash
# Run all checks with text output
./chaoslabs-doctor

# Output JSON for automation
./chaoslabs-doctor --format json

# Save results to file
./chaoslabs-doctor --format json-pretty --output results.json
```

### Integration in Code

```go
import "fraware/chaos-controller"

// Before running experiments
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

### Health Check Integration

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

## 🐳 Container Deployment

### Docker

```bash
# Run with necessary capabilities and mounts
docker run --cap-add=NET_ADMIN --cap-add=SYS_ADMIN \
  -v /proc:/proc -v /sys:/sys -v /dev:/dev \
  chaoslabs-agent:latest
```

### Kubernetes

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
      volumes:
      - name: proc
        hostPath:
          path: /proc
      - name: sys
        hostPath:
          path: /sys
      - name: dev
        hostPath:
          path: /dev
```

## 🧪 Testing

### Run Tests

```bash
# Run all tests
make test

# Run preflight tests only
make test-preflight

# Run with race detection
make test-race

# Run with coverage
make test-coverage
```

### Test Specific Components

```bash
# Test preflight system
go test -v -run "TestPreflight" ./...

# Test with benchmarks
go test -v -bench=. -benchmem ./...

# Test specific function
go test -v -run "TestCheckCapability" ./...
```

## 📊 Monitoring

### Prometheus Metrics

```go
// Track preflight check results
var (
    preflightChecksTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "preflight_checks_total",
            Help: "Total number of preflight checks",
        },
        []string{"status", "check_type"},
    )
)
```

### Alerting Rules

```yaml
# Prometheus alert
groups:
- name: preflight
  rules:
  - alert: PreflightChecksFailed
    expr: preflight_checks_total{status="fail"} > 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "Agent preflight checks failed"
```

## 🚨 Troubleshooting

### Common Issues

| Issue | Solution |
|-------|----------|
| `tc: command not found` | `sudo apt-get install iproute2` |
| `Operation not permitted` | `sudo setcap cap_net_admin+ep /path/to/agent` |
| `Module not found` | `sudo modprobe sch_netem ifb` |
| Container permissions | Run with `--cap-add=NET_ADMIN` |

### Debug Mode

```bash
# Enable debug output
export CHAOSLABS_DEBUG=1

# Run with verbose logging
./chaoslabs-doctor --verbose
```

### Check Logs

```bash
# View agent logs
journalctl -u chaoslabs-agent -f

# Check system capabilities
cat /proc/$$/status | grep CapEff

# Verify kernel modules
lsmod | grep -E "(netem|ifb)"
```

## 📈 Performance

### Execution Times

- **Total preflight**: <25ms
- **Capability checks**: <1ms
- **Tool availability**: <5ms
- **System requirements**: <10ms
- **Network capabilities**: <5ms

### Optimization

```go
// Cache results for performance
type CachedPreflightManager struct {
    *PreflightManager
    cache     *PreflightResult
    cacheTime time.Time
    cacheTTL  time.Duration
}

// Run in background
go func() {
    for {
        result, err := preflightManager.RunAllChecks()
        // Store result for quick access
        time.Sleep(5 * time.Minute)
    }
}()
```

## 🔒 Security

### Best Practices

- Grant only necessary capabilities
- Use principle of least privilege
- Regularly audit capability assignments
- Avoid running with `--privileged` when possible
- Implement proper resource limits

### Capability Matrix

| Capability | Required | Purpose |
|------------|----------|---------|
| CAP_NET_ADMIN | ✅ | Network fault injection |
| CAP_SYS_ADMIN | ⚠️ | Advanced stress testing |
| CAP_SYS_RESOURCE | ⚠️ | Resource management |

## 📚 Documentation

- [Full Preflight Documentation](docs/AGENT_PREFLIGHT_CHECKS.md)
- [Agent Architecture](docs/AGENT_ARCHITECTURE.md)
- [Troubleshooting Guide](docs/TROUBLESHOOTING.md)
- [API Reference](docs/API_REFERENCE.md)

## 🤝 Contributing

### Adding New Checks

1. Implement check logic in `preflight.go`
2. Add tests in `preflight_test.go`
3. Update documentation
4. Include remediation steps
5. Add metrics and monitoring

### Code Style

```bash
# Format code
make format

# Run linting
make lint

# Run full checks
make full-check
```

## 📞 Support

- **Issues**: [GitHub Issues](https://github.com/your-repo/issues)
- **Discussions**: [GitHub Discussions](https://github.com/your-repo/discussions)
- **Documentation**: [docs/](docs/) directory
- **Examples**: [examples/](examples/) directory

---

**Quick Reference**: `make doctor` - Run preflight checks  
**Full Docs**: See [docs/AGENT_PREFLIGHT_CHECKS.md](docs/AGENT_PREFLIGHT_CHECKS.md)
