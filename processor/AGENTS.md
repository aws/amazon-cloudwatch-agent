# processor/ — Custom OTel Processors

## What This Is

Custom OpenTelemetry processors for the CloudWatch Agent. Currently contains only the rollup processor.

## rollupprocessor/

Aggregates metrics by rolling up datapoints with different attribute combinations into a single metric with multiple attribute sets. Uses a TTL cache to track which attribute combinations have been seen.

Key files:
- `processor.go` — Core logic. Iterates metric datapoints and groups them by cache key.
- `cache.go` — TTL-based cache for tracking attribute rollup groups. Has a `nopRollupCache` for when rollup is disabled.
- `factory.go` — Standard OTel processor factory pattern.

## Note on Other Processors

Most CW Agent processors live in `plugins/processors/` (Telegraf-style) rather than here. The `plugins/processors/` directory contains:
- `ec2tagger` — Tags metrics with EC2 instance metadata
- `k8sdecorator` — Enriches metrics with Kubernetes metadata (pod cache TTL 2min)
- `ecsdecorator` — Enriches metrics with ECS metadata (reads from ECS Agent API port 51678 + cgroup filesystem)
- `gpuattributes` — GPU-specific attribute processing
- `awsneuron` — AWS Neuron (Inferentia/Trainium) attribute processing
- `awsapplicationsignals` — Application Signals processing with Count-Min Sketch cardinality control (default 500 unique combinations, 1-hour rotation). Rollup: `LocalOperation` → `"AllOtherOperations"`, `RemoteOperation` → `"AllOtherRemoteOperations"`. Rule actions: keep, drop, replace.
- `awsentity` — Entity attribute processing
- `kueueattributes` — Kueue workload queue attributes

The split is historical: `processor/` is for pure OTel processors, `plugins/processors/` is for processors that originated in the Telegraf era but have been adapted.
