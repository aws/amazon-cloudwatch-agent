# Amazon CloudWatch Agent

## What This Is

A Go application that wraps both Telegraf and the OpenTelemetry Collector into a unified telemetry agent for AWS. It collects metrics, logs, and traces from EC2 instances, EKS clusters, ECS containers, and on-premises servers, then ships them to CloudWatch, Amazon Managed Prometheus, and X-Ray.

## Architecture — The Two Runtimes

This agent runs TWO collection runtimes in a single process:

1. **Telegraf runtime** — Legacy metric/log collection via input/output plugins (`plugins/`). Configured via JSON → TOML translation (`translator/`).
2. **OTel Collector runtime** — Modern pipeline-based collection via receivers, processors, exporters, extensions. Configured via YAML injected through `CW_OTEL_CONFIG_CONTENT` env var.

The Telegraf adapter (`receiver/adapter/`) bridges the two: it wraps Telegraf input plugins as OTel receivers so they can feed into OTel pipelines.

## Critical Boundaries

- `cmd/amazon-cloudwatch-agent/` is the main entry point. It boots both runtimes.
- `service/defaultcomponents/components.go` is the OTel component registry. Every custom receiver, processor, exporter, and extension must be registered here via `otelcol.MakeFactoryMap`.
- `translator/` converts user-facing JSON config into internal TOML (for Telegraf) and YAML (for OTel). The `Rule` interface (`translator/rule.go`) is the core abstraction — each config key has a Rule that transforms it.
- `plugins/plugins.go` is the Telegraf plugin registry. Blank imports activate plugins.

## What Must Never Happen

- Don't add an OTel component without registering it in `service/defaultcomponents/components.go` — it will silently not load.
- Don't add a Telegraf plugin without a blank import in `plugins/plugins.go` — same result.
- Don't break the `receiver/adapter/` bridge — it's the only path for Telegraf inputs to emit OTel metrics.
- Don't assume OTel-only or Telegraf-only. Both runtimes coexist and the agent must support both config paths.

## Build

`make build` builds the agent. See `Makefile` for Docker and platform-specific targets.

## Config Reload & OTel Injection

OTel config is injected via `env:CW_OTEL_CONFIG_CONTENT` — this is how Helm charts, systemd units, and ECS task definitions pass config. The main agent has a `reloadLoop` that restarts the OTel collector on config changes.

## Key Dependencies

- Go 1.25.8 (uses `go.mod` with extensive `replace` directives — **do not run `go mod tidy` without understanding the replace blocks**, they pin forked dependencies)
- Forked `amazon-contributing/opentelemetry-collector-contrib` (NOT upstream `open-telemetry/opentelemetry-collector-contrib` — PRs go to the fork first)
- Forked `aws/telegraf` (NOT upstream `influxdata/telegraf`)
- AWS SDK v1 and v2 (both used — v1 for legacy Telegraf plugins, v2 for newer OTel components and extensions)

## Directory Map

The codebase splits across two paradigms:
- **Telegraf side**: `plugins/` (inputs/outputs/processors), `plugins/plugins.go` (registry)
- **OTel side**: `receiver/`, `processor/`, `extension/`, `service/defaultcomponents/components.go` (registry)
- **Bridge**: `receiver/adapter/` (Telegraf → OTel), `plugins/outputs/cloudwatch/` (has both interfaces)
- **Config**: `translator/` (JSON → TOML + YAML), `cfg/` (AWS credentials), `cmd/` (binary entry points)
- **Shared**: `internal/` (utilities), `handlers/` (SDK request handlers)
- **Packaging**: `packaging/`, `tool/`, `licensing/`

## Downlinks

- Config translation deep dive: `translator/AGENTS.md`
- OTel component registry (the most important file for OTel): `service/AGENTS.md`
- Telegraf plugin registry: `plugins/AGENTS.md`
- Custom OTel receivers (including the Telegraf adapter bridge): `receiver/AGENTS.md`
- Custom OTel extensions (entitystore, agenthealth, k8smetadata): `extension/AGENTS.md`
- Shared internal utilities: `internal/AGENTS.md`
