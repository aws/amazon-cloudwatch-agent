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

Prefer `go get module@version` then `go mod tidy`. Keep dependency PRs focused on `go.mod` / `go.sum`.

### AWS SDK for Go v2 (CVE-2026-89090 / GHSA-xmrv-pmrh-hhx2)

EventStream decoder panic fixed in the 2026-03-23 SDK release. Minimum floors for packages used here:

| Module | Minimum |
| --- | --- |
| `aws/protocol/eventstream` | `v1.7.8` |
| `service/cloudwatchlogs` | `v1.65.0` |
| `service/kinesis` | `v1.43.5` |
| `service/s3` | `v1.97.3` |

Bump those (and related direct SDK clients) together so smithy/core stay aligned. Do not vendor unless the project already vendors.

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
go list -m github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream \
  github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs \
  github.com/aws/aws-sdk-go-v2/service/kinesis \
  github.com/aws/aws-sdk-go-v2/service/s3
go build ./cmd/amazon-cloudwatch-agent
```

## Conventions

- Match existing package structure and log prefixes (`I!`, `E!`, `D!`).
- Prefer small, testable helpers over expanding `runAgent`.
- Keep PRs focused; do not reformat unrelated files.
- After edits, run `make fmt` then the affected unit tests.
