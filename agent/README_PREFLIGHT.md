# Agent preflight and doctor tool

The agent includes **preflight** checks (see `preflight.go`) and a small **doctor** CLI under `agent/cmd/doctor` to validate the environment before running fault injection.

## Quick start

From the repository root:

```bash
cd agent/cmd/doctor
go run .
```

Or build a binary:

```bash
cd agent/cmd/doctor
go build -o chaoslabs-doctor .
./chaoslabs-doctor
```

Optional flags (if implemented in `main.go`): `--format json`, `--verbose`, etc.

To run agent tests (including preflight tests):

```bash
cd agent
go test -race ./...
```

## What is validated

Typical checks include:

- **Capabilities:** `CAP_NET_ADMIN`, `CAP_SYS_ADMIN`, `CAP_SYS_RESOURCE` where applicable  
- **Tools:** `tc`, `ip`, `stress-ng`, cgroup utilities, `ifconfig` or `ip`  
- **Container:** Privileges and mounts when running under Docker or Kubernetes  
- **System:** Kernel modules (e.g. `sch_netem`, `ifb`), resources  
- **Network:** Interfaces usable for `tc` / netem  

Exact behavior is defined in `preflight.go` and tests in `preflight_test.go`.

## Prerequisites on Linux

```bash
# Debian/Ubuntu
sudo apt-get install -y iproute2 stress-ng cgroup-tools net-tools

# RHEL/CentOS
sudo yum install -y iproute-tc stress-ng libcgroup-tools net-tools
```

Load modules when needed:

```bash
sudo modprobe sch_netem
sudo modprobe ifb
```

## Containers

Agents usually need elevated privileges for `tc` and stress tools, for example:

```bash
docker run --cap-add=NET_ADMIN --cap-add=SYS_ADMIN \
  -v /proc:/proc -v /sys:/sys -v /dev:/dev \
  chaoslabs/chaos-agent:latest
```

Kubernetes: see `infrastructure/k8s/agent-deployment.yaml` and [docs/KUBERNETES.md](../docs/KUBERNETES.md).

## Troubleshooting

| Symptom | What to try |
|--------|-------------|
| `tc: command not found` | Install `iproute2` / `iproute-tc` |
| Operation not permitted | Add `NET_ADMIN` (and often `SYS_ADMIN`) or run privileged where appropriate |
| Missing module | `modprobe sch_netem` / `ifb` |

More: [docs/TROUBLESHOOTING.md](../docs/TROUBLESHOOTING.md).

## Module path

The doctor module is `github.com/fraware/chaoslabs/cmd/doctor` (see `agent/cmd/doctor/go.mod`). It is listed in the repo root `go.work`.

## Contributing

When adding checks, update tests in `preflight_test.go` and mention new requirements here or in [docs/TROUBLESHOOTING.md](../docs/TROUBLESHOOTING.md).
