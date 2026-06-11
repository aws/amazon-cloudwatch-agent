# OpenTelemetry Collect Pipeline Architecture

## Feature Pipelines (data sources)

| Component | host_insights | prometheus | otlp | database_insights |
|-----------|:---:|:---:|:---:|:---:|
| **Receivers** | hostmetrics | prometheus | otlp/grpc, otlp/http | postgresql (metrics), postgresql (events), filelog/postgresql |
| **Processors** | — | transform/set_cluster_name (optional) | — | filter/exclude_monitor, transform/resource, transform/fix_start_time, transform/logs, resourcedetection |
| **Exporters** | forward/opentelemetry | forward/opentelemetry | forward/opentelemetry | forward/opentelemetry, count/dbi_dbload, signaltometrics/dbi_topsql |
| **Connectors** | forward/opentelemetry | forward/opentelemetry | forward/opentelemetry | forward/opentelemetry, count/dbi_dbload, signaltometrics/dbi_topsql |
| **Extensions** | — | — | — | — |
| **Pipelines** | metrics/host_insights | metrics/otel_prometheus | metrics/otlp, logs/otlp, traces/otlp | metrics/dbi (raw→connectors), metrics/dbi (connectors→fwd), logs/dbi_events, logs/dbi_server_logs |

## Base Export Pipelines (shared infrastructure)

| Component | metrics/opentelemetry | logs/opentelemetry | traces/opentelemetry |
|-----------|:---:|:---:|:---:|
| **Receivers** | forward/opentelemetry | forward/opentelemetry | forward/opentelemetry |
| **Processors** | resourcedetection, batch/opentelemetry | resourcedetection, attributestocontext, transform/logs_cleanup, batch/logs | resourcedetection, batch/opentelemetry |
| **Exporters** | otlphttp/metrics | otlphttp/logs | otlphttp/traces |
| **Extensions** | sigv4auth/monitoring, agenthealth/otlphttp_metrics | sigv4auth/logs, awscloudwatchlogsprovisioner, headerssetter/logs, agenthealth/logs | sigv4auth/xray, agenthealth/traces |
| **Endpoint** | monitoring.{region}.amazonaws.com/v1/metrics | logs.{region}.amazonaws.com/v1/logs | xray.{region}.amazonaws.com/v1/traces |

## Data Flow

```
host_insights (metrics) ───┐
prometheus (metrics) ──────┤──→ forward/opentelemetry ──→ metrics/opentelemetry ──→ CloudWatch
otlp (metrics) ────────────┤
dbi (metrics via connectors)┘

otlp (logs) ───────────────┐
dbi (events) ──────────────┤──→ forward/opentelemetry ──→ logs/opentelemetry ──→ CloudWatch Logs
dbi (server-logs) ─────────┘

otlp (traces) ─────────────────→ forward/opentelemetry ──→ traces/opentelemetry ──→ X-Ray
```

## DBI Internal Sub-Pipelines Detail

```
postgresql/metrics ──→ filter/exclude_monitor ──→ count/dbi_dbload ──→ ┐
                                                  signaltometrics/topsql┤──→ transform/resource
                                                                       │    transform/fix_start_time
                                                                       └──→ forward/opentelemetry

postgresql/events ──→ filter ──→ resourcedetection ──→ transform/resource ──→ transform/logs ──→ forward/opentelemetry

filelog/postgresql ──→ resourcedetection ──→ transform/resource ──→ transform/logs ──→ forward/opentelemetry
```
