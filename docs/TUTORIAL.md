# ChaosLabs tutorial

This guide walks through common chaos experiments against a running controller and agent.

## Prerequisites

- Controller on `http://localhost:8080` and agent reachable from the controller (see [Setup & Installation](../README.md#setup--installation)).
- Docker Compose or local `go run` for both services.
- Optional: dashboard at [http://localhost:3000](http://localhost:3000) (`dashboard-v2` dev server or Compose `dashboard` service).

## Tutorial 1: CPU stress

### Step 1: Create the request

`cpu_stress.json`:

```json
{
  "name": "CPU Stress Test",
  "description": "Stress test with 4 CPU workers for 15 seconds",
  "experiment_type": "cpu-stress",
  "duration": 15,
  "cpu_workers": 4
}
```

### Step 2: Start the experiment

```bash
curl -X POST -H "Content-Type: application/json" -d @cpu_stress.json http://localhost:8080/start
```

### Step 3: Monitor

- Controller and agent logs (JSON logs on the agent; structured **slog** on the controller).
- Dashboard at http://localhost:3000 (if running).
- Metrics: `http://localhost:8080/metrics` and `http://localhost:9090/metrics`.

## Tutorial 2: Network latency

`network_latency.json`:

```json
{
  "name": "Network Latency Test",
  "description": "Simulate 100ms network latency for 30 seconds",
  "experiment_type": "network-latency",
  "duration": 30,
  "delay_ms": 100
}
```

```bash
curl -X POST -H "Content-Type: application/json" -d @network_latency.json http://localhost:8080/start
```

Confirm in agent logs that `tc` / netem changes were applied and reverted. Use Grafana if you import the repo’s dashboard JSON.

## Tutorial 3: Memory stress and process kill

### Memory stress

`mem_stress.json`:

```json
{
  "name": "Memory Stress Test",
  "description": "Allocate 200 MB for 30 seconds",
  "experiment_type": "mem-stress",
  "duration": 30,
  "mem_size_mb": 200
}
```

```bash
curl -X POST -H "Content-Type: application/json" -d @mem_stress.json http://localhost:8080/start
```

### Process kill

`process_kill.json`:

```json
{
  "name": "Process Kill Test",
  "description": "Kill a process matching 'go'",
  "experiment_type": "process-kill",
  "kill_process": "go"
}
```

```bash
curl -X POST -H "Content-Type: application/json" -d @process_kill.json http://localhost:8080/start
```

## Scheduling and parallel runs

- **Scheduled:** add `start_time` in RFC3339 format to the JSON body.
- **Parallel:** set `"parallel": true` and `"agent_count"` to fan out to multiple agents.

## Next steps

- Tune experiments and watch Prometheus metrics.
- Configure **OTLP** tracing (`OTEL_EXPORTER_OTLP_ENDPOINT`) per [TROUBLESHOOTING.md](TROUBLESHOOTING.md).
- Report issues or ideas via GitHub (see [CONTRIBUTING.md](CONTRIBUTING.md)).
