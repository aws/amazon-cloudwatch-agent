# CloudWatch Agent

Go agent that collects metrics, logs, and traces for Amazon CloudWatch. It translates JSON into Telegraf TOML plus an OpenTelemetry collector YAML, then runs both pipelines in one process.

## Layout

- `cmd/amazon-cloudwatch-agent` — main process
- `translator/translate/logs` — JSON `timestamp_format` to TOML `timestamp_regex` / `timestamp_layout`
- `plugins/inputs/logfile` — parses and optionally trims the timestamp from each log line

## Log timestamp formats

`timestamp_format` is strftime. `%f` is fractional seconds and already includes the decimal point in the Go layout (`.999999999`).

A format of `.%f` must stay a single dot. Turning it into `..000` makes `time.Parse` miss lines such as `2026-02-04T11:27:44.945314+00:00`, so CloudWatch gets the wrong timestamp and `trim_timestamp` never runs (#2000).

Literal regex characters in the format (`+`, `.`) belong in `timestamp_regex` only. The Go layout keeps them literal.

## Commands

```sh
go test ./translator/translate/logs/logs_collected/files/collect_list ./plugins/inputs/logfile
```

## Conventions

- Match existing package structure and log prefixes (`I!`, `E!`, `D!`, `W!`).
- Keep PRs focused; do not reformat unrelated files.
