# CLAUDE.md

Guidance for AI coding agents working in this repository.

## Project overview

Amazon CloudWatch Agent (CWA) collects system metrics, logs, and traces and publishes them to Amazon CloudWatch. It embeds Telegraf and OpenTelemetry Collector components, plus CloudWatch-specific adapters and translators.

Upstream: https://github.com/aws/amazon-cloudwatch-agent

## Architecture (high level)

1. **JSON agent config** → translator (`translator/`) produces Telegraf TOML and/or OTEL YAML.
2. **Runtime** (`cmd/amazon-cloudwatch-agent`) loads plugins via blank imports in `plugins/plugins.go`.
3. **Telegraf inputs** run behind OTEL receiver adapters (`receiver/adapter`).
4. **Windows vs Linux**: most Windows object names under `metrics_collected` become `win_perf_counters`. True Telegraf Windows plugins must be allowlisted (see checklist below).
5. **Logs**: file and Windows Event Log `collect_list` entries share destination fields (`log_group_name`, `log_stream_name`, `retention_in_days`, `timezone`). Schema `additionalProperties: false` rejects unknown keys—add properties to `translator/config/schema.json` when extending collect_list.

## Common commands

```bash
go test ./translator/translate/logs/logs_collected/windowsevents/collect_list/...
go test ./translator/cmdutil/ -run 'WindowsEvents|LogWindows'
go test ./translator/config/...
make build
```

## Adding or extending log collect_list fields

1. Add the property under the matching definition in `translator/config/schema.json` (classic `logsWindowsEventsDefinition` and OTEL `opentelemetry.collect.windows_events` when relevant).
2. Add a translator child rule under `translator/translate/logs/logs_collected/.../collect_list/` (mirror existing `rule*.go` files).
3. If the field must survive TOML round-trip, add it to the plugin config struct and `tomlConfigTemplate` types.
4. Extend `translator/config/sampleSchema/` samples and package unit tests.

## Safety / contribution norms

- Do not commit secrets, credentials, or customer data.
- Keep PRs focused; avoid drive-by refactors.
- Follow existing copyright headers (`// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.` / MIT SPDX).
- Contributors generally cannot merge to `aws/amazon-cloudwatch-agent`; open a PR for maintainer review. A CLA may be requested for larger changes (see `CONTRIBUTING.md`).

## Issue context

Timezone on `windows_events.collect_list` was requested in https://github.com/aws/amazon-cloudwatch-agent/issues/127. File logs already support `timezone` (`UTC` / `Local` / `LOCAL`); Windows events rejected it via schema. Event Log `SystemTime` is already absolute UTC for CloudWatch publication; the field provides config parity with file logs.
