# CloudWatch Agent

Go agent that collects metrics, logs, and traces for Amazon CloudWatch. It translates JSON (and optional YAML) into Telegraf TOML plus an OpenTelemetry collector YAML, then runs both pipelines in one process.

## Layout

- `cmd/amazon-cloudwatch-agent` — main process (Telegraf + OTEL collector)
- `cmd/start-amazon-cloudwatch-agent` — translates config, then execs the agent
- `cmd/config-translator` / `tool/translator` — JSON → TOML/YAML
- `translator/` — translation rules (metrics, logs, traces, Container Insights)
- `plugins/`, `receiver/`, `processor/`, `exporter/`, `extension/` — Telegraf and OTEL components
- `service/configprovider` — OTEL config loading (`-otelconfig`, `file:`, `env:`)

## Dependencies

Prefer `go get module@version` then `go mod tidy`. Keep dependency PRs focused on `go.mod` / `go.sum`. Avoid unbounded `@latest` bumps that drag `k8s.io/client-go` past the collector-contrib pin.

### go.opentelemetry.io/otel (CVE-2026-41178 / GHSA-5wrp-cwcj-q835)

Baggage parsing DoS in **exactly** `v1.41.0` and `v1.43.0`. Fixed in `v1.42.0` / `v1.44.0+`. Keep `go.opentelemetry.io/otel` at ≥ `v1.44.0` (currently `v1.47.0`) and leave `exclude` entries for the two vulnerable releases.

When bumping otel, update related `metric` / `trace` / `sdk` / OTLP exporter modules together, then re-pin `k8s.io/api`, `k8s.io/apimachinery`, and `k8s.io/client-go` if tidy moves them.

## Commands

```sh
make test
make fmt
make lint
make build
```

```sh
go list -m go.opentelemetry.io/otel
go build ./cmd/amazon-cloudwatch-agent
```

## Conventions

- Match existing package structure and log prefixes (`I!`, `E!`, `D!`).
- Prefer small, testable helpers over expanding `runAgent`.
- Keep PRs focused; do not reformat unrelated files.
