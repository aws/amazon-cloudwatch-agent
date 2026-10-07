# CloudWatch Agent

Go agent that collects metrics, logs, and traces for Amazon CloudWatch. It translates JSON (and optional YAML) into Telegraf TOML plus an OpenTelemetry collector YAML, then runs both pipelines in one process.

## Layout

- `cmd/amazon-cloudwatch-agent` — main process (Telegraf + OTEL collector)
- `cmd/start-amazon-cloudwatch-agent` — translates config, then execs the agent
- `packaging/dependencies/amazon-cloudwatch-agent.service` — systemd unit
- `translator/`, `plugins/`, `receiver/`, `processor/`, `exporter/`, `extension/`

## systemd unit

Ship `KillMode=mixed` (not `process`). With Prometheus/AppInsights configs, glib/godbus can auto-launch `dbus-daemon --session`. Under `KillMode=process` those children survive `systemctl stop`/`restart`, accumulate inotify watches, and eventually fail with `too many open files` (#2195).

`mixed` sends SIGTERM to the main process, then SIGKILL to remaining cgroup members so leaked session buses are reaped on stop.

Keep the regression test in `packaging/dependencies/amazon_cloudwatch_agent_service_test.go` green.

## Commands

```sh
make test
go test ./packaging/dependencies
```

## Conventions

- Match existing package structure and log prefixes (`I!`, `E!`, `D!`, `W!`).
- Keep PRs focused; do not reformat unrelated files.
