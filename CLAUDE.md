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

Prefer `go get module@version` then `go mod tidy`. For CVE floors on transitive modules, use `exclude` for vulnerable releases and keep a patched version selected (see `google.golang.org/grpc` excludes for CVE-2026-84303 / 84304 / 84445; require ≥ 1.83.2, currently 1.84.0).

Do not vendor unless the project already vendors. Keep dependency PRs focused on `go.mod` / `go.sum` (and excludes when locking a security floor).

## Config translation

`config-translator` writes `amazon-cloudwatch-agent.toml` always. It writes `amazon-cloudwatch-agent.yaml` only when at least one OTEL pipeline exists. If every OTEL pipeline is disabled, the YAML is omitted on purpose; the agent must still start (logs-only / Fluent Bit sidecar).

## Commands

```sh
make test          # unit tests
make fmt           # goimports + license headers
make lint          # golangci-lint
make build         # cross-compile agent binaries
```

Focused check while iterating:

```sh
go test ./cmd/amazon-cloudwatch-agent ./internal/util/config ./tool/translator
go list -m google.golang.org/grpc
```

## Conventions

- Match existing package structure and log prefixes (`I!`, `E!`, `D!`).
- Prefer small, testable helpers over expanding `runAgent`.
- Keep PRs focused; do not reformat unrelated files.
- After edits, run `make fmt` then the affected unit tests.
