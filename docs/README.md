# ChaosLabs documentation

| Document | Description |
|----------|-------------|
| [../README.md](../README.md) | Project overview, quick start, usage |
| [ARCHITECTURE.md](ARCHITECTURE.md) | Components, repo layout, observability |
| [KUBERNETES.md](KUBERNETES.md) | Deploying on Kubernetes |
| [TUTORIAL.md](TUTORIAL.md) | Step-by-step experiment examples |
| [TROUBLESHOOTING.md](TROUBLESHOOTING.md) | Common failures (ports, tracing, faults, metrics) |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to contribute, style, tests |
| [windows-setup.md](windows-setup.md) | Windows 10/11 dev environment |
| [EVENT_BUS_IMPLEMENTATION.md](EVENT_BUS_IMPLEMENTATION.md) | NATS JetStream event bus design |
| [api/openapi.yaml](api/openapi.yaml) | Controller HTTP API (OpenAPI 3) |
| [../Makefile](../Makefile) | Local checks: `make verify`, `tidy`, `integration-test` |

## Component-specific

| Path | Topic |
|------|--------|
| [../controller/HTTP_CONFIGURATION.md](../controller/HTTP_CONFIGURATION.md) | HTTP hardening, env vars, metrics |
| [../agent/README_PREFLIGHT.md](../agent/README_PREFLIGHT.md) | Agent preflight / doctor checks |
| [../cli/README.md](../cli/README.md) | CLI usage |
| [../bench/README.md](../bench/README.md) | Performance benchmarks |
| [../bench/BASELINE_README.md](../bench/BASELINE_README.md) | Baseline targets and methodology |
| [../tests/integration/README.md](../tests/integration/README.md) | Integration tests (Redis/NATS) |
