# Troubleshooting

More context: [README.md](../README.md), [KUBERNETES.md](KUBERNETES.md), and [controller/HTTP_CONFIGURATION.md](../controller/HTTP_CONFIGURATION.md).

## Controller or agent will not start

- Confirm the configured ports are free (`8080` controller, `9090` agent).
- For Docker Compose, ensure the agent service uses build target `runtime` and the controller uses `production` (see `infrastructure/compose/docker-compose.yml` or the root compose wrapper).

## Tracing export errors

- Traces use **OTLP over HTTP**. Set `OTEL_EXPORTER_OTLP_ENDPOINT` (for example `http://jaeger:4318` or your OpenTelemetry Collector URL).
- Use `OTEL_EXPORTER_OTLP_INSECURE=true` when the endpoint is plain HTTP.
- Optional dependency health: set `OTEL_HEALTHCHECK_URL` on the controller to an HTTP URL that returns 200 when the collector is healthy.

## Fault injection does not apply

- The agent typically needs **privileged** access or capabilities such as `NET_ADMIN` / `SYS_ADMIN` for `tc` and related tools.
- Verify `AGENT_ENDPOINTS` on the controller points at reachable agent `/inject` URLs.

## Metrics not scraped

- Ensure Prometheus scrape annotations match the listen ports (`8080` controller, `9090` agent).
- Hit `/metrics` directly from inside the cluster to verify exposition.

## HTTP API errors

- Public routes are described in [api/openapi.yaml](api/openapi.yaml).
- Validation errors return JSON with `error` / `message` fields; the controller sets **`X-Request-Id`** on responses (and accepts it on incoming requests).
