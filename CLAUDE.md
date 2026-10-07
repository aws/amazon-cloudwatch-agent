# CloudWatch Agent

Go agent that collects metrics, logs, and traces for Amazon CloudWatch. It translates JSON (and optional YAML) into Telegraf TOML plus an OpenTelemetry collector YAML, then runs both pipelines in one process.

## Layout

- `cmd/amazon-cloudwatch-agent` — main process (Telegraf + OTEL collector)
- `cmd/start-amazon-cloudwatch-agent` — translates config, then execs the agent
- `cmd/config-translator` / `tool/translator` — JSON → TOML/YAML
- `translator/` — translation rules (metrics, logs, traces, Container Insights)
- `plugins/`, `receiver/`, `processor/`, `exporter/`, `extension/` — Telegraf and OTEL components
- `service/configprovider` — OTEL config loading (`-otelconfig`, `file:`, `env:`)

## Windows performance counters

`plugins/inputs/win_perf_counters` scrapes PDH counters. After Windows Update / MSI reconfiguration, PDH handles can go stale (`#2199`).

`Gather()` must not swallow total failure:
- Log `W!` on the first all-counters failure, then `E!` after consecutive empty scrapes (see `gather_health.go`).
- Return an error when every configured counter fails to init/collect so `receiver/adapter` marks the scrape failed.
- `item.reset()` closes PDH handles so the next scrape re-opens them (recovery without reboot when PDH is healthy again).

Pure helpers in `gather_health.go` are OS-agnostic and must stay covered by unit tests that run on Linux CI.

## Commands

```sh
make test
make fmt
make lint
go test ./plugins/inputs/win_perf_counters
```

## Conventions

- Match existing package structure and log prefixes (`I!`, `E!`, `D!`, `W!`).
- Prefer small, testable helpers over expanding `Gather`.
- Keep PRs focused; do not reformat unrelated files.
