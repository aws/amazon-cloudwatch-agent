# receiver/ — Custom OTel Receivers

## What This Is

Custom OpenTelemetry receivers built specifically for the CloudWatch Agent. These are registered in `service/defaultcomponents/components.go`.

## Receivers

| Directory | Purpose |
|-----------|---------|
| `adapter/` | Wraps Telegraf input plugins as OTel receivers. This is the bridge between the legacy Telegraf runtime and the OTel pipeline. |
| `awsnvmereceiver/` | Collects NVMe device metrics (throughput, IOPS, latency) from local NVMe devices. Detects EBS vs Instance Store via magic numbers in sysfs metadata. |
| `systemmetricsreceiver/` | Collects system-level metrics (CPU, memory, disk, network) as an OTel-native alternative to Telegraf system plugins. |

## The Adapter Pattern (adapter/)

This is the most architecturally significant receiver. It allows any Telegraf input plugin to emit metrics into OTel pipelines:

```
Telegraf Input Plugin → OtelAccumulator → consumer.Metrics → OTel Pipeline
```

The adapter is NOT statically registered in `components.go` — it dynamically creates receiver factories per Telegraf input plugin via `Type(input)`.

## Adding a New Custom Receiver

1. Create directory under `receiver/`
2. Implement `factory.go` with `NewFactory()` returning `receiver.Factory`
3. Implement `config.go` with config struct and `Validate()`
4. Implement the receiver logic
5. Register in `service/defaultcomponents/components.go`

## What Must Never Happen

- Don't break the adapter accumulator — it's the only way Telegraf plugins produce OTel metrics.
- Don't change the `Type()` function signature in `adapter/factory.go` — it generates component type names that appear in OTel configs.
