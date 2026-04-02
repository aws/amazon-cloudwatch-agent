# cmd/ — Binary Entry Points

## What This Is

Each subdirectory builds a separate binary. The agent ships multiple executables for different lifecycle stages.

## Binaries

| Directory | Binary | Purpose |
|-----------|--------|---------|
| `amazon-cloudwatch-agent/` | `amazon-cloudwatch-agent` | Main agent process. Boots Telegraf + OTel runtimes. |
| `start-amazon-cloudwatch-agent/` | `start-amazon-cloudwatch-agent` | Launcher that sets up paths and env vars, then execs the main agent. |
| `config-translator/` | `config-translator` | Converts user JSON config → internal TOML config. Runs before agent starts. |
| `config-downloader/` | `config-downloader` | Downloads config from SSM Parameter Store or S3. Runs before translator. |
| `amazon-cloudwatch-agent-config-wizard/` | `config-wizard` | Interactive CLI wizard for generating agent JSON config. |
| `workload-discovery/` | `workload-discovery` | Discovers running workloads for Application Signals. |
| `xray-migration/` | `xray-migration` | Migrates X-Ray daemon config to CW Agent config format. |

## Startup Sequence

```
config-downloader → config-translator → start-amazon-cloudwatch-agent → amazon-cloudwatch-agent
```

1. `config-downloader` fetches JSON config from SSM/S3/local file
2. `config-translator` converts JSON → TOML (Telegraf) + YAML (OTel)
3. `start-amazon-cloudwatch-agent` sets up environment and execs the agent
4. `amazon-cloudwatch-agent` loads both configs and starts collection

## Key Patterns

- The main agent (`amazon-cloudwatch-agent.go`) has a `reloadLoop` that watches for config changes and restarts the OTel collector. Config is injected via `env:CW_OTEL_CONFIG_CONTENT`.
- The `components()` function merges default OTel factories with Telegraf adapter factories.
- Windows support uses `kardianos/service` for Windows Service integration.
- The `merge.go` file handles merging multiple JSON config files (e.g., from SSM + local). Customers use multi-file configs in production — this is a critical path.

## What Must Never Happen

- Don't change the startup sequence — downstream tooling (systemd units, Windows services, SSM documents, ECS task definitions, Helm charts) depends on this exact binary chain.
- Don't remove the config merge capability — customers use multi-file configs in production.
- Don't change the `env:CW_OTEL_CONFIG_CONTENT` URI scheme — it's how all container deployments inject OTel config.
