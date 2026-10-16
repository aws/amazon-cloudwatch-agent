# CloudWatch Agent

Go agent that collects metrics, logs, and traces for Amazon CloudWatch. It translates JSON (and optional YAML) into Telegraf TOML plus an OpenTelemetry collector YAML, then runs both pipelines in one process.

## Layout

- `cmd/amazon-cloudwatch-agent` — main process (Telegraf + OTEL collector)
- `cmd/start-amazon-cloudwatch-agent` — translates config, then execs the agent
- `translator/`, `plugins/`, `receiver/`, `processor/`, `exporter/`, `extension/`

## AWS SDK for Go v1

`github.com/aws/aws-sdk-go` v1 is end of support (2025-07-31). The agent module must stay on the final v1 release, `v1.55.8`, so the binary does not ship the old `v1.48.x` pin (#2090).

New AWS API code uses `github.com/aws/aws-sdk-go-v2`. Do not add a lower v1 requirement. `internal/awssdk/v1_version_test.go` locks the selected module and `aws.SDKVersion`.

`tool/clean` is a separate module and must not require unused `aws-sdk-go` v1.

## Commands

```sh
go test ./internal/awssdk
go test ./tool/clean/...
```

## Conventions

- Match existing package structure and log prefixes (`I!`, `E!`, `D!`, `W!`).
- Keep PRs focused; do not reformat unrelated files.
