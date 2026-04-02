# extension/ — Custom OTel Extensions

## What This Is

OTel extensions provide cross-cutting capabilities that aren't part of the metric/log/trace pipeline. They run alongside pipelines and provide services to other components.

## Extensions

### agenthealth/
Reports the agent's own health status to CloudWatch. Intercepts AWS SDK calls to track success/failure rates, latency, and status codes, marshaled into a header sent with every API call.

Key patterns:
- Two separate flag systems: stats flags (`handler/stats/agent/flag.go`) and user-agent feature flags (`handler/useragent/`). Don't confuse them — they serve different purposes.
- Supports an optional additional auth extension for chained authentication.

### entitystore/
Stores entity metadata (EC2 instance info, EKS pod-service mappings) for use by other components. Acts as a shared state store within the agent process.

Key patterns:
- `EC2Info` — Fetches and caches instance ID and account ID from EC2 metadata.
- `eksInfo` — Maintains a TTL cache mapping pod names to service/environment names for Application Signals.
- Singleton pattern via `GetEntityStore()` — other components access the shared instance.

Service Name Resolution Priority (for log file entity attribution, from `logFileServiceAttribute`):
1. Log group-level attributes (set via `addEntryForLogGroup`)
2. Log file-level attributes (set via `addEntryForLogFile`)
3. ResourceTags — EC2 instance tags checked in order: `service` > `application` > `app`
4. ClientIamRole (IAM role name)
5. AutoScalingGroup (environment only, not service name)
6. Fallback → `"unknown_service"`

For non-logfile paths (`getServiceNameAndSource`), the chain is shorter: IMDS tags → IAM role → fallback.

Additional source: `K8sWorkload` — used when Kubernetes workload metadata is available.

Caching: EKS pod mappings — TTL 5min, 256 entries. EC2 metadata — retry every 1 min.

### k8smetadata/
Provides Kubernetes pod metadata lookup by IP address or service name. Used by processors that need to enrich metrics with pod-level information.

Key patterns:
- Singleton pattern via `GetKubernetesMetadata()`.
- Includes jitter on startup to avoid thundering herd when all DaemonSet pods start simultaneously.
- Delayed deletion: 2 minutes — don't assume metadata disappears immediately when pods terminate.

### server/
HTTP server extension for the agent (health endpoints, debug endpoints).

## Factory Pattern

All extensions follow the same pattern:
1. `factory.go` — `NewFactory()` returns `extension.Factory`
2. `config.go` — Config struct with `Validate()`
3. `extension.go` — Implements `extension.Extension` with `Start()` and `Shutdown()`
4. Singleton accessor (e.g., `GetEntityStore()`) for cross-component access

## What Must Never Happen

- Don't create circular dependencies between extensions — they start in registration order, not dependency order.
- Don't block in `Start()` — extensions must start quickly or the agent hangs on boot.
- Don't access singleton extensions before `Start()` is called — the data won't be populated yet.
