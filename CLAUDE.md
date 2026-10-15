# CloudWatch Agent

Go agent that collects metrics, logs, and traces for Amazon CloudWatch. It translates JSON (and optional YAML) into Telegraf TOML plus an OpenTelemetry collector YAML, then runs both pipelines in one process.

## Layout

- `cmd/amazon-cloudwatch-agent` — main process (Telegraf + OTEL collector)
- `cmd/start-amazon-cloudwatch-agent` — translates config, then execs the agent
- `cmd/config-translator` / `tool/translator` — JSON → TOML/YAML
- `translator/` — translation rules (metrics, logs, traces, Container Insights)
- `plugins/`, `receiver/`, `processor/`, `exporter/`, `extension/` — Telegraf and OTEL components
- `service/configprovider` — OTEL config loading (`-otelconfig`, `file:`, `env:`)

## Feature gates

The agent builds collector args itself (`--config=…`) and does not expose a general `--feature-gates` passthrough.

`cmd/amazon-cloudwatch-agent/featuregates.go` enables `service.profilesSupport` by default so profiles pipelines can start (see #2273 / #2274). Only gates that are still registered and not deprecated are passed; once a gate graduates and is removed upstream, the agent skips it instead of failing startup.

When adding another default gate, append its ID to `defaultEnabledFeatureGates` and cover registry states in `featuregates_test.go` (alpha, stable, absent, deprecated) plus a GlobalRegistry smoke check.

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
go test ./cmd/amazon-cloudwatch-agent -run FeatureGate
```

## Conventions

- Match existing package structure and log prefixes (`I!`, `E!`, `D!`).
- Prefer small, testable helpers over expanding `runAgent`.
- Keep PRs focused; do not reformat unrelated files.
- After edits, run `make fmt` then the affected unit tests.
