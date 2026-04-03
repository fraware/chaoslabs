# Docker Compose layouts

All Compose files assume the **project directory is the repository root** so build contexts and bind mounts resolve correctly.

```bash
# Default minimal stack (also available via root docker-compose.yml)
docker compose --project-directory . -f infrastructure/compose/docker-compose.yml up --build

# Full development stack (hot reload, Redis, NATS, observability sidecars)
docker compose --project-directory . -f infrastructure/docker-compose.dev.yml up --build

# CI-style dependencies (Redis, NATS, mock API)
docker compose --project-directory . -f infrastructure/docker-compose.test.yml up -d
```

| File | Purpose |
|------|---------|
| [docker-compose.yml](docker-compose.yml) | Controller, agent, dashboard only |
| [../docker-compose.dev.yml](../docker-compose.dev.yml) | Development dependencies and tooling |
| [../docker-compose.test.yml](../docker-compose.test.yml) | Integration / smoke test dependencies |
