# plugins/ — Telegraf-Style Plugins

## What This Is

Input, output, and processor plugins that follow the Telegraf plugin model. These are the legacy collection path — newer functionality should use OTel receivers/processors/exporters instead.

## Plugin Registration

`plugins.go` activates plugins via blank imports. If a plugin isn't imported here, it won't be available at runtime. This file is the Telegraf equivalent of `service/defaultcomponents/components.go`.

Two categories of blank imports:
- **CW Agent plugins** (in this repo): `logfile`, `nvidia_smi`, `prometheus`, `statsd`, `win_perf_counters`, `windows_event_log`, `cloudwatch`, `cloudwatchlogs`, `ecsdecorator`, `k8sdecorator`
- **Upstream Telegraf plugins** (from forked `aws/telegraf`): `cpu`, `disk`, `diskio`, `ethtool`, `mem`, `net`, `processes`, `procstat`, `socket_listener`, `swap`

## Subdirectories

- `inputs/` — `logfile`, `nvidia_smi`, `prometheus`, `statsd`, `win_perf_counters` (Windows-only), `windows_event_log` (Windows-only)
- `outputs/` — `cloudwatch` (metrics), `cloudwatchlogs` (logs)
- `processors/` — `ec2tagger`, `k8sdecorator`, `ecsdecorator`, `gpuattributes`, `awsneuron`, `awsapplicationsignals`, `awsentity`, `kueueattributes`, `nodemetadataenricher`

## Key Patterns

- Input plugins implement `telegraf.Input` or `telegraf.ServiceInput`.
- Output plugins implement `telegraf.Output`.
- The `cloudwatch/` output has both a Telegraf interface AND an OTel exporter interface (`convert_otel.go`, `factory.go`) — it bridges both worlds.
- Processor plugins in `processors/` are registered separately from `plugins.go` — they're loaded by the OTel component system, not Telegraf's plugin registry.

## What Must Never Happen

- Don't remove blank imports from `plugins.go` without verifying no customer config depends on that plugin.
- Don't assume all plugins run on all platforms — `win_perf_counters` and `windows_event_log` are Windows-only.
- Don't modify the `cloudwatch/` output without understanding both its Telegraf and OTel interfaces.

## Critical Pitfalls

### CloudWatch Logs Output — Concurrency Trap
- Default `concurrency=1`: each log destination (group+stream) gets its own goroutine. Destinations are isolated — throttling on one does NOT block others. No head-of-line blocking.
- `concurrency > 1`: creates a SHARED WorkerPool. A slow/throttled destination CAN starve others. This INTRODUCES head-of-line blocking — the opposite of what users expect.
- **Never recommend increasing concurrency to fix API throttling.** Recommend service limit increases or log filtering instead.

### Logfile Plugin — fd_release + auto_removal Log Loss
- When both `fd_release` and `auto_removal` are enabled, `fd_release` can release file descriptors before `auto_removal` cleans up, causing the agent to lose its read position and miss log data.
- **Avoid enabling both simultaneously** — their interaction can cause unexpected log gaps.

### CloudWatch Metrics Output — Dimension Rollup
The `cloudwatch/` output supports `rollup_dimensions` config — a list of dimension key sets. For each metric, the output publishes additional copies with only the specified dimension subsets. This is configured via `aggregation_dimensions` in the user JSON config. Changing this logic affects how many metric datums get published per input metric.

### CloudWatch Metrics Output — High-Resolution Metrics
When `metrics_collection_interval` < 60 seconds, the output automatically sets `StorageResolution=1` (high-resolution). This is detected via the `aws:StorageResolution` metric tag. High-resolution metrics have different pricing — don't change interval detection logic without understanding the billing impact.

### CloudWatch Metrics Output — Key Limits
Max 1,000 datums per API call, 999 KB payload, 30 dimensions per metric. 10 concurrent publishers with 10,000 metric buffer. See constants in `cloudwatch.go`.

### CloudWatch Logs Output — Key Limits
1 MB batch size, 10,000 events, 24-hour time span, 5-second default flush. See constants in `pusher/batch.go`.

### Retry Strategy (Logs)
Dual strategy in `pusher/retry.go`: SHORT (200ms base) for most errors, LONG (2s base) for 500/503 and throttling.
