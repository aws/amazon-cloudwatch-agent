# CloudWatch Agent

Go agent that collects metrics, logs, and traces for Amazon CloudWatch. It translates JSON (and optional YAML) into Telegraf TOML plus an OpenTelemetry collector YAML, then runs both pipelines in one process.

## Layout

- `cmd/amazon-cloudwatch-agent` — main process (Telegraf + OTEL collector)
- `cmd/start-amazon-cloudwatch-agent` — translates config, then execs the agent
- `cmd/config-translator` / `tool/translator` — JSON → TOML/YAML
- `translator/` — translation rules (metrics, logs, traces, Container Insights)
- `plugins/`, `receiver/`, `processor/`, `exporter/`, `extension/` — Telegraf and OTEL components
- `service/configprovider` — OTEL config loading (`-otelconfig`, `file:`, `env:`)

## Config translation

`config-translator` writes `amazon-cloudwatch-agent.toml` always. It writes `amazon-cloudwatch-agent.yaml` only when at least one OTEL pipeline exists (traces, EMF, Container Insights, Application Signals). If every OTEL pipeline is disabled, the YAML is omitted on purpose.

The agent must start without that YAML. Logs-only EKS add-on setups keep Fluent Bit in a sidecar; the agent process still has to stay up.

Do not pass a missing path as `-otelconfig`. Skip missing file URIs; if none remain, run without OTEL pipelines instead of failing in the collector config provider.

## Commands

```sh
make test          # unit tests
make fmt           # goimports + license headers
make lint          # golangci-lint
make build         # cross-compile agent binaries
```

Run a focused package while iterating:

```sh
go test ./cmd/amazon-cloudwatch-agent ./internal/util/config ./tool/translator
```

## Conventions

- Match existing package structure and log prefixes (`I!`, `E!`, `D!`).
- Prefer small, testable helpers over expanding `runAgent`.
- Keep PRs focused; do not reformat unrelated files.
- After edits, run `make fmt` then the affected unit tests.
