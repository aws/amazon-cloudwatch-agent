# CloudWatch Agent

Go agent that collects metrics, logs, and traces for Amazon CloudWatch. It translates JSON (and optional YAML) into Telegraf TOML plus an OpenTelemetry collector YAML, then runs both pipelines in one process.

## Layout

- `cmd/amazon-cloudwatch-agent` — main process (Telegraf + OTEL collector)
- `cmd/start-amazon-cloudwatch-agent` — translates config, then execs the agent
- `packaging/dependencies/amazon-cloudwatch-agent.service` — systemd unit
- `translator/`, `plugins/`, `receiver/`, `processor/`, `exporter/`, `extension/`

## Host custom metrics entity association

Statsd/collectd (`metrics/hostCustomMetrics`) must use the **Resource** entity processor (`awsentity/resource`), same as host metrics — not `awsentity/service/telegraf` with `scrape_datapoint_attribute: true`.

Service entity + scrape required Application Signals service metadata that plain custom metrics never carry. From agent `1.300031+` that silently dropped all statsd datapoints before `PutMetricData` with no error (#2180).

Keep `translator/translate/otel/pipeline/host` and the `tocwconfig` golden YAMLs aligned with Resource association for custom metrics.

## systemd unit

Ship `KillMode=mixed` (not `process`). With Prometheus/AppInsights configs, glib/godbus can auto-launch `dbus-daemon --session`. Under `KillMode=process` those children survive `systemctl stop`/`restart`, accumulate inotify watches, and eventually fail with `too many open files` (#2195).

## Commands

```sh
make test
go test ./translator/translate/otel/pipeline/host ./translator/tocwconfig
```

## Conventions

- Match existing package structure and log prefixes (`I!`, `E!`, `D!`, `W!`).
- Keep PRs focused; do not reformat unrelated files.
