# translator/ — Config Translation Engine

## What This Is

Converts user-facing JSON configuration into internal formats: TOML for the Telegraf runtime and YAML for the OTel Collector runtime. This is the bridge between "what the customer writes" and "what the agent understands."

## Core Abstractions

Two translation systems coexist:
- **Legacy `Rule` interface** (`rule.go`) — transforms JSON keys into TOML for Telegraf. Applied recursively via `processRuleToApply.go`.
- **Modern `ComponentTranslator`** (`translate/otel/common/common.go`) — generic type alias `Translator[component.Config, component.ID]` that transforms JSON into OTel YAML.

New features should only need a ComponentTranslator. Rules are for backward compatibility with the Telegraf path.

## Subdirectories

Key directories: `tocwconfig/` (TOML/YAML output, 229 test fixtures in `sampleConfig/`), `translate/` (orchestration, `translate/otel/pipeline/` has per-pipeline translators), `jsonconfig/` (parsing), `registerrules/` (rule registration), `config/` (structs + schema.json), `context/` (OS/mode/credentials).

## Translation Flow

```
JSON config → jsonconfig/ (parse + merge) → registerrules/ (load rules) → tocwconfig/ (apply rules) → TOML output (Telegraf)
                                                                        → translate/otel/ (pipeline translators) → YAML output (OTel)
```

## Pipeline → Component Mapping

| Pipeline | Receivers | Processors | Exporters |
|----------|-----------|------------|-----------|
| host | adapter | ec2tagger, awsentity | cloudwatch |
| host/delta_metrics | adapter, awsnvme | cumulativetodelta | cloudwatch |
| systemmetrics | hostmetricsreceiver | ec2tagger, awsentity | cloudwatch |
| containerinsights | containerinsights | batch, filter, gpuattributes | awsemf |
| applicationsignals | otlp | resourcedetection, awsapplicationsignals | awsemf, awsxray |
| jmx | jmx | filter, metricstransform | awsemf |
| prometheus (CW) | adapter (telegraf_prometheus) | batch | awsemf |
| prometheus (AMP) | prometheusreceiver | batch, deltatocumulative | prometheusremotewrite |
| xray | awsxray, otlp | batch | awsxray |

## Prometheus — Two Completely Different Pipelines

Prometheus metrics can be published to CloudWatch OR Amazon Managed Prometheus (AMP). These use entirely different pipelines — modifying one does NOT affect the other:

| Aspect | CloudWatch Path | AMP Path |
|--------|----------------|----------|
| Config section | `metrics.metrics_collected.prometheus` | `metrics.metrics_collected.prometheus` + `metrics_destinations.amp` |
| Receiver | adapter (telegraf_prometheus) | prometheusreceiver (OTel native) |
| Processing | batch | batch + deltatocumulative |
| Exporter | awsemfexporter | prometheusremotewriteexporter |
| Auth | agenthealth | sigv4auth |

## EMF Log Group Naming Patterns

| Environment | Log Group |
|-------------|-----------|
| ECS | `/aws/ecs/containerinsights/{ClusterName}/performance` |
| EKS | `/aws/containerinsights/{ClusterName}/performance` |
| Prometheus | `/aws/containerinsights/{ClusterName}/prometheus` |
| App Signals | `/aws/application-signals/data` |

## What Must Never Happen

- Don't change the JSON config schema without updating the corresponding translation rules — customers depend on backward compatibility.
- Don't assume Linux-only — the translator must produce valid config for all supported OS platforms.
- Don't modify `tocwconfig/` without understanding the rule chain — rules have ordering dependencies.
