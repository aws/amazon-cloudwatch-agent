# AWS Neuron Processor

The AWS Neuron Processor enriches and normalizes metrics from the [AWS Neuron Monitor](https://awsdocs-neuron.readthedocs-hosted.com/en/latest/tools/neuron-sys-tools/neuron-monitor-user-guide.html) exporter on AWS Inferentia and Trainium instances. It ensures a complete, consistent metric set is always emitted — even when no Neuron workload is running.

| Status                   |                           |
| ------------------------ |---------------------------|
| Stability                | [beta]                    |
| Supported pipeline types | metrics                   |
| Distributions            | [amazon-cloudwatch-agent] |

## Overview

The Neuron Monitor Prometheus exporter emits per-core and per-node metrics only when a Neuron runtime is active. On idle nodes, only the `neuron_hardware_info` topology metric is present. This processor detects the idle state and synthesizes zero-valued datapoints for all expected metrics so downstream consumers (CloudWatch, Prometheus, dashboards) always see a complete metric set.

## Behavior

The processor performs three operations on each metric batch:

### 1. Synthesize missing metrics for idle nodes

When `neuron_hardware_info` (or `neuron_hardware`) is present but expected metrics are absent, the processor creates zero-valued datapoints for all 10 expected metrics:

**Per-core metrics** (one datapoint per core):
| Metric | Type |
|--------|------|
| `neuroncore_utilization_ratio` | Gauge |
| `neuroncore_memory_usage_constants` | Gauge |
| `neuroncore_memory_usage_model_code` | Gauge |
| `neuroncore_memory_usage_model_shared_scratchpad` | Gauge |
| `neuroncore_memory_usage_runtime_memory` | Gauge |
| `neuroncore_memory_usage_tensors` | Gauge |

**Per-node metrics** (one datapoint per variant):
| Metric | Type | Variants |
|--------|------|----------|
| `execution_status_total` | Sum (monotonic) | 6 status types |
| `execution_errors_total` | Sum (monotonic) | 5 error types |
| `neuron_runtime_memory_used_bytes` | Gauge | 2 memory locations |
| `execution_latency_seconds` | Gauge | 7 percentiles |

### 2. Add `neurondevice` attribute

Real metrics from Neuron Monitor carry a `neuroncore` index but no device index. The processor computes `neurondevice = floor(neuroncore / neuroncore_per_device_count)` and adds it to every datapoint that has a `neuroncore` attribute.

### 3. Scale utilization to percent

`neuroncore_utilization_ratio` arrives as a 0.0–1.0 ratio from the exporter. The processor multiplies by 100 to produce a 0–100 percentage, matching the Container Insights V1 convention.

## Passthrough behavior

- If no `neuron_hardware_info` or `neuron_hardware` metric is in the batch, all metrics pass through unmodified. Non-Neuron nodes are unaffected.
- If a metric already exists in the batch, it is not duplicated — only missing metrics are synthesized.

## Configuration

The processor has no configuration options. It is enabled by including it in the pipeline:

```yaml
processors:
  awsneuron:

service:
  pipelines:
    metrics:
      processors: [awsneuron]
```

## Instance labels

Synthesized datapoints inherit instance-level labels from the `neuron_hardware_info` metric:

- `availability_zone`
- `instance_id`
- `instance_name`
- `instance_type`
- `region`
- `subnet_id`

## Hardware topology

The processor reads topology from `neuron_hardware_info` datapoint attributes:

| Attribute | Description |
|-----------|-------------|
| `neuron_device_count` | Number of Neuron devices on the instance |
| `neuroncore_per_device_count` | Number of NeuronCores per device |

If either attribute is missing, per-core metrics are not synthesized (per-node metrics are still synthesized).
