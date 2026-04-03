# Integration tests

Black-box checks against Redis and NATS. Part of the CI workflow; see [docs/README.md](../../docs/README.md) for other documentation. Run locally or in CI with services up:

```bash
export REDIS_URL=redis://127.0.0.1:6379
export NATS_URL=nats://127.0.0.1:4222
go test -tags=integration -v -race ./...
```

Or use `docker compose --project-directory . -f infrastructure/docker-compose.test.yml up -d` and the defaults above.
