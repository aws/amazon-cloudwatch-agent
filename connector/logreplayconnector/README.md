# Log Replay Connector

| Status                   |                     |
| ------------------------ | ------------------- |
| Stability                | [alpha]: logs to logs |
| Distributions            | CloudWatch agent    |

[alpha]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/component-stability.md#alpha

The `log_replay` connector keeps the latest log record that matches a rule for each entity, for example for each
container. At each interval, it sends an exact copy of every kept record into its logs pipeline. Only the timestamp
and the observed timestamp of a copy change, to the time of the copy.

Some facts appear in a log only one time. For example, an inference engine such as vLLM writes one startup line
with its release, its model, and its effective configuration. A reader that queries the last hour of logs does not
find that line for an engine that started days ago. The log retention can also delete it. With this connector, every
window of one interval holds a copy of the line while the engine runs and the copies are sent.

The connector does not send the original record. The logs pipeline that feeds the connector exports the original
record with its other exporters.

## Configuration

| Key | Default | Description |
| --- | --- | --- |
| `interval` | `1h` | How often a copy of each kept record is sent. From `1m` to `24h`. |
| `source_file_attribute` | `log.file.path` | A log record attribute with the path of the file that the record came from. A record without it is not kept. At each tick, a kept record whose file no longer exists is removed instead of sent. The file is the only sign that the source of a record stopped, so the key must not be empty. The filelog receiver sets `log.file.path` with `include_file_path: true`. |
| `storage` | none | The ID of a storage extension, for example `file_storage/state`. With it, the kept records survive a restart. |
| `max_state_bytes` | `4194304` (4 MiB) | The size of all kept records together. From 64 KiB to 64 MiB. |
| `max_record_bytes` | `65536` (64 KiB) | The size of one kept record, as OTLP JSON with its resource and scope. A larger record is not kept. From 1 KiB to 1 MiB, and at most `max_state_bytes`. |
| `error_mode` | `ignore` | How an OTTL evaluation error of a condition is handled: `ignore`, `silent`, or `propagate`. With `propagate`, the connector stops checking the batch at the first error and keeps the matches found before it. It returns the error to the component before it in the logs pipeline. A batch processor there logs the error and drops it. A receiver that gets the error and retries sends the batch again to every exporter of the pipeline. |
| `rules` | required | One or more rules. |

Each rule has these keys:

| Key | Default | Description |
| --- | --- | --- |
| `name` | required | The name of the rule in the stored state and in the logs. Each rule needs its own name. |
| `body_contains` | none | Substrings. To match, a record must have a string body that contains at least one of them. This check does not use OTTL and runs first. |
| `conditions` | none | OTTL conditions in the log context, ORed. To match, a record must make at least one of them true. A rule needs `body_contains`, `conditions`, or both. If both are set, a record must pass both. |
| `identity` | required | The resource attribute keys that identify an entity. The rule keeps the latest matching record for each distinct set of values. |
| `max_entries` | `500` | The number of records that the rule keeps. From 1 to 100000. |

## Example

This example keeps the engine startup line of each vLLM container on a Kubernetes node, and sends a copy of it every
hour.

```yaml
extensions:
  file_storage/state:
    directory: /var/lib/amazon-cloudwatch-agent/otel-state

connectors:
  log_replay/vllm:
    storage: file_storage/state
    interval: 1h
    source_file_attribute: log.file.path
    rules:
      - name: vllm_engine_config
        body_contains:
          - "Initializing an LLM engine (v"
          - "Initializing a V0 LLM engine (v"
          - "Initializing a V1 LLM engine (v"
        identity: [k8s.namespace.name, k8s.pod.name, k8s.container.name]

service:
  extensions: [file_storage/state]
  pipelines:
    logs:
      # The filelog receiver reads /var/log/containers/*.log with include_file_path: true. It sets
      # k8s.namespace.name, k8s.pod.name, and k8s.container.name from the file name. The example does not show
      # its configuration.
      receivers: [filelog]
      processors: [k8sattributes, batch]
      exporters: [otlphttp/logs, log_replay/vllm]
    logs/replay:
      receivers: [log_replay/vllm]
      processors: [batch]
      exporters: [otlphttp/logs]
```

The copies go to the same exporter as the original records. So, if the exporter sets the log destination, a copy
lands in the same place as the original record.

The kubelet writes the log of each container to `/var/log/pods`, and it adds a symbolic link to that file in
`/var/log/containers`. When the kubelet removes a container, it removes the link too. Thus, the copies of a container
stop within one interval after the kubelet removes the container. The connector checks the link itself, not its
target, because the target is missing for a moment while the kubelet rotates the log. A new pod with the name of a
previous pod is the same entity: its startup line replaces the kept record of the previous pod.

A copy has the same content as the original record. Only its two times differ: in a copy, the timestamp and the
observed timestamp are equal. Thus, a metric filter, a subscription filter, an alarm, or a count on the line counts
each copy too. That is one more match per container and interval. To leave out the copies, compare the two times of
each event. The start time of the pod gives the start time of the engine.

## Behavior

### Matching

For each record and rule, the connector first does the `body_contains` check. Most records fail this check, and the
check does not allocate. Then it evaluates the conditions. A rule without `body_contains` evaluates its conditions on
every record. To be kept, a matching record must have a value that is not empty for every identity attribute of its
resource. The record must also have the source file attribute.

The connector keeps a copy of the record with its resource and its scope. The newest record of an entity is its kept record.
A new record with the same time as the kept record also replaces it. The time of a record is its timestamp, else its observed timestamp, else the
current time. A time later than the current time counts as the current time. Within one batch, the records apply in
order, so of two records of an entity with the same time, the last one wins. One record can match more than one rule,
and each rule keeps it. A matching record that is larger than `max_record_bytes` is not kept. If it is not older
than the kept record of its entity, it removes that record, which is stale.

Each change of the kept records rewrites the whole stored state, at most one time each second. So a rule must match
rare records, such as a startup line.

The connector does not change its input, so the pipeline does not copy the data for it.

### Copies

The first tick comes 10 seconds after the start, plus a random delay of less than 10 seconds and less than the
interval. The random delay spreads the ticks of agents that start at the same time. After the first tick, a tick
comes at each interval. A record kept between two ticks gets its first copy at the next tick. With `storage`, the
stored state holds the time of the latest copies. After a restart, the first tick waits until one interval after
them. Thus, a restart does not add copies, except after a failed or canceled call (see below), or a stop before
the state is saved.

At each tick, the connector checks the source file of each kept record. It uses `lstat`. Thus, a symbolic link
counts as present, also with a missing target:

- If the file does not exist, the connector checks it again after one second. A rotation that renames a file and
  then makes a new one leaves it missing for a moment. If the file is still missing, the connector removes the
  record and does not send it.
- If the connector cannot check the file for another reason, for example a missing permission, it does not send the
  record. The record stays kept for the next tick.

The connector sends all other records to the next consumer, sorted by rule and then by identity. Each call holds
at most 1 MiB of records. If the next consumer returns an error, the connector logs it at most one time each minute.
The error does not go back to the logs pipeline that feeds the connector, and the records stay kept for the next
tick. If one call of a tick fails, the time of the copies is not stored, also for the calls that succeeded. So after a
restart, those copies go out again at the first tick.

At shutdown, the connector stops the ticks and cancels a tick in progress. It waits for that tick for half of the
time left before the shutdown deadline, or for 5 seconds without a deadline. With `storage`, it then saves the state.
After the save, Shutdown waits until the tick ends or the deadline passes. The batch processor finishes its calls
despite the cancellation. If the next consumer does so before the save, the stored state holds the time of that
tick. Otherwise, the copies of that tick go out again after a restart.

### Persistence

If `storage` is set, the connector keeps its records in the storage extension. The stored state is one JSON record
with the key `state` and the version `1`. The connector reads it one time at startup, and after that only writes it, within about
one second after a change. A change is a new, replaced, or removed record, and a tick whose copies the next
consumer accepted. The stored state holds the time of that tick, for the first tick after a restart.

At startup, the connector checks each stored record against the current configuration. It restores only the records
that pass all of these checks:

- The rule of the record still exists, and the record matches it.
- The identity of the record is the same.
- The record has the source file attribute.
- The record fits `max_record_bytes`.

A stored time later than the current time counts as the current time. The first tick after a restart sends the
copies of the restored records.

With `file_storage`, the state is one file in its directory, `connector_logreplay_<name>`, from the type `log_replay`
without `_` and the name of the connector. The file holds the kept log records as they are, so it holds log
content of the node.

If the storage extension does not exist or is not a storage extension, the connector does not start. If the
extension cannot open its storage, the connector logs one Error and keeps the records in memory only. It does not
try to open the storage again. If the connector cannot read the stored state, it logs a Warn and keeps the records in
memory only until the next restart. Thus, the stored state stays for the next start.

A damaged file can also stop the open or the first read. bbolt reads its file through a memory map. On Linux and
macOS, a read of a missing part of the file makes a memory fault instead of an error. The connector turns such a fault or
a panic into an error while it opens the storage and reads the stored state. So the agent keeps running. With
`recreate: true`, `file_storage` renames a file that makes bbolt panic or fault while bbolt opens it. It renames the
file to a `.backup` file and starts a new one. A fault in the compaction of `compaction.on_start` comes later, so
that open fails and the file stays. Other damaged files also stay. If the open fails, the connector logs an Error at
each start. If the read fails, it logs a Warn. In both cases, it keeps the records in memory only. Some damage makes
bbolt panic on a goroutine of its own, and that crashes the agent at each start. To start again with an empty state,
delete the file.

### Limits

If a rule already keeps `max_entries` records, a new record of the rule removes the rule's record with the oldest
time. A new record that is older than all kept records of the rule is not stored. If the kept records are larger than `max_state_bytes`, the connector removes the records with the oldest time,
of any rule. It never removes the new record. The size of a kept record is the size of its JSON form in the
stored state: the record plus its key, its identity, its source file, and about 100 bytes. Thus, keep
`max_record_bytes` well below `max_state_bytes`. When the two are equal, a record at `max_record_bytes` does not fit.

## Logs

- Warn, at most one time each minute for each kind: records removed for `max_entries` or `max_state_bytes`, a
  matching record larger than `max_record_bytes`, a source file that cannot be checked, and an error of the next
  consumer.
- Error, one time at startup: the storage cannot open.
- Error, at most one time each minute: the stored state cannot be encoded or written.
- Warn, one time at startup: the stored state cannot be read.
- Info, one time at startup with `storage`: the number of restored, dropped, and changed records.
- Other logs about single records are at the Debug level. With `error_mode: ignore`, OTTL logs each evaluation error
  of a condition at the Warn level.
