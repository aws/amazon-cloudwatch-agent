# internal/ — Shared Utilities

## What This Is

Internal packages shared across the agent. These are not importable by external Go modules (Go's `internal/` convention).

## Key Packages

Notable packages (see directory listing for full set):
- `containerinsightscommon/` — Shared vocabulary for Container Insights metrics. Heavily imported by both this repo and the OTel Contrib fork.
- `ec2metadataprovider/` — EC2 IMDS client (v1+v2) with caching and request deduplication. **All components must use this — don't create new IMDS clients** or you'll hit rate limits.
- `detector/` — Platform detection (EC2, ECS, EKS, on-premises) AND workload detection (tomcat, kafka, java, nvidia) for Application Signals.
- `state/` — Log file offset tracking with B-tree range merging and truncation detection via sequence numbers.
- `retryer/` — Dual retry strategy: `LogThrottleRetryer` (SHORT 200ms / LONG 2s) and `IMDSRetryer`.
- `mapWithExpiry/` — TTL-based map used extensively for K8s metadata caching.

## What Must Never Happen

- Don't move packages out of `internal/` — external consumers would break (and the Go compiler enforces this anyway).
- Don't duplicate constants — use `containerinsightscommon/` and `constants/` as the single source of truth.
- Don't create new EC2 metadata clients — use `ec2metadataprovider/` to avoid IMDS rate limiting.
