# service/ — OTel Component Registration and Config

## What This Is

The glue layer that connects the OTel Collector framework to all CW Agent components. This is where the agent's identity as an OTel Collector is defined.

## Subdirectories

### defaultcomponents/
**This is the most important file in the agent for OTel integration.**

`components.go` contains `Factories()` which returns all registered OTel component factories (69 total: 22 receivers, 28 processors, 8 exporters, 11 extensions). Read the file for the full list.

If a component isn't registered here, it doesn't exist in the agent.

### configprovider/
Handles OTel config loading and validation:
- `provider.go` — Creates `ConfigProviderSettings` from URI list. Supports `env:` scheme for loading config from environment variables.
- `otlphttp_validator.go` — Security validator that ensures OTLP/HTTP exporters only send to AWS endpoints. Builds an allowlist dynamically from `endpoints.DefaultPartitions()` DNS suffixes plus `"api.aws"`. This is a security boundary — removing it would allow data exfiltration to non-AWS endpoints.
- `flags.go` — Custom flag type for OTel config URIs.

### registry/
Component registry utilities.

## What Must Never Happen

- Don't forget to register new components in `defaultcomponents/components.go` — this is the #1 cause of "my new receiver doesn't work."
- Don't remove the OTLP/HTTP validator — it prevents the agent from accidentally exporting telemetry to non-AWS endpoints, which is a security boundary.
- Don't change the config provider URI scheme — `env:CW_OTEL_CONFIG_CONTENT` is how the Helm chart and systemd units inject config.
