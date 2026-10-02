# CLAUDE.md

Guidance for AI coding agents working in this repository.

## Project overview

Amazon CloudWatch Agent (CWA) collects system metrics, logs, and traces and publishes them to Amazon CloudWatch. It embeds Telegraf and OpenTelemetry Collector components, plus CloudWatch-specific adapters and translators.

Upstream: https://github.com/aws/amazon-cloudwatch-agent

## Architecture (high level)

1. **JSON agent config** → translator (`translator/`) produces Telegraf TOML and/or OTEL YAML.
2. **Runtime** (`cmd/amazon-cloudwatch-agent`) loads plugins via blank imports in `plugins/plugins.go`.
3. **Telegraf inputs** run behind OTEL receiver adapters (`receiver/adapter`).
4. **Windows vs Linux**: most Windows object names under `metrics_collected` become `win_perf_counters`. True Telegraf Windows plugins must be:
   - blank-imported in `plugins/plugins.go`
   - registered with `RegisterWindowsRule` under `translator/translate/metrics/metrics_collect/<plugin>/`
   - blank-imported in `translator/registerrules/register_rules.go`
   - listed in `DisableWinPerfCounters` (`translator/translate/metrics/config/registered_metrics.go`)
   - listed in `windowsInputSet` (`translator/translate/otel/receiver/adapter/translators.go`)
   - defined in `translator/config/schema.json`

Skipping the allowlists causes Windows configs to be misrouted into `win_perf_counters`.

## Common commands

```bash
# Unit tests for a package
go test ./translator/translate/metrics/metrics_collect/win_services/...
go test ./translator/translate/otel/receiver/adapter/...
go test ./translator/cmdutil/...
go test ./plugins/...

# Broader translator + receiver coverage
go test ./translator/... ./receiver/adapter/...

# Full local build (slow; cross-compiles packages)
make build
```

## Adding a Telegraf input (checklist)

1. Confirm the plugin exists in the pinned Telegraf replace in `go.mod` (`github.com/aws/telegraf`).
2. Blank-import it in `plugins/plugins.go` (prefer upstream over forking unless CWA needs custom behavior).
3. Add JSON→TOML translator under `translator/translate/metrics/metrics_collect/<name>/` (mirror `statsd` / `ethtool`).
4. Wire registration, schema, units (`receiver/adapter/accumulator/default_unit.go`), and tests.
5. Add a `translator/config/sampleSchema/valid*.json` sample and a `translator/cmdutil` schema test when introducing a new top-level metrics key.

## Safety / contribution norms

- Do not commit secrets, credentials, or customer data.
- Keep PRs focused; avoid drive-by refactors and unrelated formatting.
- Follow existing package patterns and copyright headers (`// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.` / MIT SPDX).
- Prefer unit tests that assert JSON→translated config shapes over end-to-end Windows-only integration unless you can run Windows CI.
- Contributors generally cannot merge to `aws/amazon-cloudwatch-agent`; open a PR and wait for maintainer review. A CLA may be requested for larger changes (see `CONTRIBUTING.md`).

## Issue context

`inputs.win_services` support was requested in https://github.com/aws/amazon-cloudwatch-agent/issues/120. Implementation wires the existing Telegraf plugin through CWA config translation and Windows allowlists; it does not reimplement service enumeration.
