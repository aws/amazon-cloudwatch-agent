# Signal to Info Metric Connector

The Signal to Info Metric connector makes Info metrics from facts in log records. It matches log records with
rules. It extracts facts with [OTTL] value expressions. It keeps the latest facts for each entity, for example for
each container of a pod. An Info metric is a gauge with the value `1`. Its data point attributes carry the facts.
When the facts change, the connector emits the metric. It also emits the metric at each `emit_interval`. A storage
extension can persist the state, so that the facts stay available after an agent restart.

The first use case is the vLLM engine startup line. vLLM writes this line one time for each engine start. The
line contains the engine configuration:

```
Initializing a V1 LLM engine (v0.11.0) with config: model='...', tensor_parallel_size=4, ...
```

| Status                   |                                   |
| ------------------------ | --------------------------------- |
| Stability                | alpha                             |
| Supported pipeline types | exporter: logs, receiver: metrics |
| Distributions            | amazon-cloudwatch-agent           |

The connector does not change the logs that it consumes. Thus, it can share a logs pipeline with other exporters.

## Configuration

### Connector settings

| Name                     | Description                                                                                                                         | Default           | Bounds                                  |
|--------------------------|-------------------------------------------------------------------------------------------------------------------------------------|-------------------|-----------------------------------------|
| `storage`                | ID of a storage extension that persists the state, for example `file_storage/state`. If not set, the state is in memory only.      | not set           |                                         |
| `emit_interval`          | Time between two emissions of the current Info metrics. Rules can override it.                                                      | `60s`             | `1s` to `24h`                           |
| `emit_on_change`         | If `true`, a change of facts causes an immediate emission of the Info metric.                                                       | `true`            |                                         |
| `max_state_bytes`        | Maximum size of the serialized state, in bytes. The connector uses a size estimate. Refer to [Entity state](#entity-state).          | `1048576` (1 MiB) | `65536` (64 KiB) to `67108864` (64 MiB) |
| `max_value_length`       | Maximum length in bytes of each value: the facts, the included resource attributes, and the identity.                               | `256`             | `1` to `1024` (CloudWatch limit)        |
| `error_mode`             | Action on OTTL evaluation errors at runtime: `propagate`, `ignore`, or `silent`. OTTL parse errors always stop the startup.         | `ignore`          | One of the three values.                |
| `rules`                  | The rules. Refer to the next table.                                                                                                 | none              | Minimum one rule.                       |

### Rule settings

| Name                          | Description                                                                                                                                                                       | Default   | Bounds                                                 |
|-------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------|--------------------------------------------------------|
| `name`                        | Metric name. It is also the key of the state of the rule.                                                                                                                         | none      | Required. Maximum 255 bytes. Unique across the rules.  |
| `description`                 | Metric description.                                                                                                                                                               | `""`      |                                                        |
| `unit`                        | Metric unit.                                                                                                                                                                      | `""`      |                                                        |
| `body_contains`               | Substrings to find in the body. This is a plain substring check on a string body, with no OTTL and no regex. It runs before the conditions.                                       | `[]`      | Entries must not be empty.                             |
| `conditions`                  | OTTL conditions on the log context, joined with OR.                                                                                                                               | `[]`      | Each condition must parse.                             |
| `identity`                    | Resource attribute keys that identify an entity, for example `k8s.pod.uid`. The connector keeps the latest facts for each entity.                                                | none      | Minimum one key. No empty or duplicate keys.           |
| `include_resource_attributes` | Resource attribute keys that the connector copies to the emitted metric. Because the list contains the identity keys, two entities never give the same series.                    | `[]`      | Must contain all identity keys. No empty or duplicate keys. |
| `attributes`                  | The facts to extract. Refer to the next table.                                                                                                                                    | none      | Minimum one. Maximum `max_attributes`.                 |
| `emit_interval`               | Overrides the connector `emit_interval` for this rule. `0` uses the connector value.                                                                                             | `0`       | If set: `1s` to `24h`.                                 |
| `max_entities`                | Maximum number of entities that this rule tracks. `0` uses the default.                                                                                                          | `0` (500) | If set: `1` to `100000`.                               |
| `max_attributes`              | Maximum number of attributes in this rule. `0` uses the default.                                                                                                                 | `0` (32)  | If set: `1` to `128`.                                  |

A rule must set `body_contains`, `conditions`, or both. For a record to match a rule, both of these checks must pass:

- If `body_contains` is set, the body contains at least one of the substrings.
- If `conditions` is set, at least one condition is true.

### Attribute settings

| Name            | Description                                                                                                                                      | Default | Bounds                                                  |
|-----------------|--------------------------------------------------------------------------------------------------------------------------------------------------|---------|---------------------------------------------------------|
| `key`           | Data point attribute key.                                                                                                                        | none    | Required. Unique in the rule.                           |
| `value`         | OTTL value expression for the matching record. If empty, the value is `log.attributes["<key>"]`.                                                 | `""`    | Must parse.                                             |
| `default_value` | If the value expression gives an error, `nil`, or an empty string, the connector uses this value. If empty, `optional` controls the result.     | `""`    | Maximum `max_value_length` bytes.                       |
| `optional`      | If `true`, a matching record can have no value for this fact. The emitted metric then does not have this attribute. If `false`, the connector does not use a record that has no value for this fact. | `false` | Not permitted together with `default_value`.            |

The connector gets each fact in this order: first the `value` expression, then `default_value`. If the two give
no value, `optional` controls the result.

### Label budget

Each data point has `len(attributes) + len(include_resource_attributes)` labels. The total must be 150 or less.
This is the CloudWatch limit for each data point.

### OTTL

Conditions and values use the OTTL log context with the [standard converters][OTTL functions]. Editors are not
available. You can write paths with or without the context name. For example, `body` and `log.body` are equal.
Also, `attributes["k"]` and `log.attributes["k"]` are equal. Resource attributes are `resource.attributes["k"]`.

An index into a converter result with a missing key gives an evaluation error, not `nil`. For example,
`ExtractPatterns(log.body, "x=(?P<v>\\d+)")["v"]` fails on a record that the pattern does not match. Thus,
`Coalesce` cannot supply a fallback value for it. Use `default_value` for a fallback value.

## Example

The example keeps the state in `file_storage/state` and emits every 30 seconds. Its rule is the rule in
`testdata/vllm_rule.yaml`, with all 14 facts of the vLLM engine startup line.

The identity of the rule is the namespace, the pod name, and the container name. The kubelet names the log file of
each container `/var/log/containers/<pod>_<namespace>_<container>-<id>.log`. The filelog receiver of the example
must set the three names from the file name, for example with a `regex_parser` operator on `log.file.path`. Thus,
every record has the identity, also before the k8sattributes processor knows the pod. The container name is in the identity, because two containers of one pod can each start an engine. The
`include_resource_attributes` list has the three identity keys, because the connector requires all identity keys
in this list. The list also has the pod UID, so that the pipeline can compare the UID of a series with the UID of
the live pod. The UID is not in the identity. Thus, a new pod with the name of a previous pod is the same entity.
Its startup line replaces the facts and the UID of the previous pod, and starts a new series. The k8sattributes
processor adds the UID to a record of a pod that it knows. A record without the UID gives a series without the UID.

```yaml
extensions:
  file_storage/state:
    directory: /var/lib/amazon-cloudwatch-agent/state

connectors:
  signal_to_info_metric/vllm:
    storage: file_storage/state
    emit_interval: 30s
    rules:
      - name: vllm_engine_config_info
        description: vLLM engine configuration parsed from the engine startup log line. The value is always 1.
        max_entities: 500
        # vllm_version needs the full startup phrase and ") with config: ", so a line with only the phrase gives no series.
        # A prefix check does not work, because vLLM writes "(EngineCore pid=N) INFO ..." or "INFO ..." first.
        body_contains:
          - "Initializing an LLM engine (v"
          - "Initializing a V0 LLM engine (v"
          - "Initializing a V1 LLM engine (v"
        # The filelog receiver reads these names from the container log file name, so every record has them.
        # Two containers of one pod are two entities.
        identity: [k8s.namespace.name, k8s.pod.name, k8s.container.name]
        # The pod UID is included, so that the pipeline can compare it with the live pod.
        include_resource_attributes: [k8s.namespace.name, k8s.pod.name, k8s.container.name, k8s.pod.uid]
        attributes:
          - key: vllm_version
            value: 'ExtractPatterns(log.body, "Initializing an? (?:V[01] )?LLM engine \\(v(?P<v>[^)]+)\\) with config: ")["v"]'
          # A block scalar, because the pattern contains single quotes.
          - key: model
            value: |-
              ExtractPatterns(log.body, "with config: model='(?P<v>[^']*)'")["v"]
          # A missing key in ExtractPatterns is an evaluation error, so the default applies to "Initializing an LLM engine" (v0.6.x) lines.
          - key: vllm_engine
            value: 'ExtractPatterns(log.body, "Initializing a (?P<v>V[01]) LLM engine")["v"]'
            default_value: V0
          - key: served_model_name
            value: 'ExtractPatterns(log.body, ", served_model_name=(?P<v>[^,]+),")["v"]'
            optional: true
          - key: dtype
            value: 'ExtractPatterns(log.body, ", dtype=(?:torch\\.)?(?P<v>[A-Za-z0-9_]+),")["v"]'
            optional: true
          - key: max_seq_len
            value: 'ExtractPatterns(log.body, ", max_seq_len=(?P<v>\\d+),")["v"]'
            optional: true
          - key: tensor_parallel_size
            value: 'ExtractPatterns(log.body, ", tensor_parallel_size=(?P<v>\\d+),")["v"]'
            optional: true
          - key: pipeline_parallel_size
            value: 'ExtractPatterns(log.body, ", pipeline_parallel_size=(?P<v>\\d+),")["v"]'
            optional: true
          - key: data_parallel_size
            value: 'ExtractPatterns(log.body, ", data_parallel_size=(?P<v>\\d+),")["v"]'
            optional: true
          - key: quantization
            value: 'ExtractPatterns(log.body, ", quantization=(?P<v>[A-Za-z0-9_.\\-]+),")["v"]'
            optional: true
          - key: enforce_eager
            value: 'ExtractPatterns(log.body, ", enforce_eager=(?P<v>True|False),")["v"]'
            optional: true
          - key: kv_cache_dtype
            value: 'ExtractPatterns(log.body, ", kv_cache_dtype=(?P<v>[A-Za-z0-9_]+),")["v"]'
            optional: true
          - key: enable_prefix_caching
            value: 'ExtractPatterns(log.body, ", enable_prefix_caching=(?P<v>True|False|None),")["v"]'
            optional: true
          # "None" for speculative_config=None, the method for speculative_config=SpeculativeConfig(method='mtp', ...).
          - key: speculative_method
            value: |-
              ExtractPatterns(log.body, "speculative_config=(?:SpeculativeConfig\\(method=')?(?P<v>None|[^',)]+)")["v"]
            optional: true

service:
  extensions: [file_storage/state]
  pipelines:
    logs:
      # filelog sets k8s.namespace.name, k8s.pod.name, and k8s.container.name from the log file name,
      # and k8sattributes adds k8s.pod.uid. The example does not show their configuration.
      receivers: [filelog]
      processors: [k8sattributes]
      exporters: [awscloudwatchlogs, signal_to_info_metric/vllm]
    metrics/info:
      receivers: [signal_to_info_metric/vllm]
      exporters: [otlphttp]
```

## Behavior

### Matching and facts

For each record and rule, the connector first does the `body_contains` check. A record without a string body
does not match a rule that sets `body_contains`. Most records fail this check, and the check does not allocate
memory. Then the connector evaluates the `conditions`. Then it reads the identity from the resource attributes,
one time for each resource and rule. If an identity key is missing or its value is empty, the rule does not use
records of that resource. Then the connector gets the facts in the configured order.

Values that are not strings are converted as by `pcommon.Value.AsString`. For example, the integer 4 becomes `"4"`.
The connector replaces each sequence of bytes that is not valid UTF-8 with the character U+FFFD, also in the
restored state. Without this replacement, a receiver that checks UTF-8 can reject the batch that holds such a
value. CloudWatch PromQL does not accept U+FFFD in a query string. To match a label value with U+FFFD, use a
regular expression, for example `.` in place of the character.
After the replacement, each value, also each resource attribute and identity value, is cut to `max_value_length`
bytes. The cut is at a UTF-8 character boundary. A value that is empty after the cut counts as missing.

With `error_mode: propagate`, the connector stops the batch at an evaluation error. This applies to the
conditions, and to a required attribute without `default_value`. The connector returns the error. It still
applies the records before that record. With `ignore` or `silent`, a condition with an evaluation error counts as
false, and the other conditions still run. An attribute with an evaluation error has no value, and then
`default_value` and `optional` apply. The connector does not change the logs that it consumes.

### Entity state

The connector keeps the latest facts for each rule and identity. The record time is the record timestamp, else
the observed timestamp, else the current time. A record time that is later than the current time counts as the
current time. For a known entity:

- If the record is older than the stored facts, the connector does not use it. After a container restart,
  kubelet keeps both log files, and the receiver can read them in any order.
- If the facts and the included resource attributes are the same, the entity stays the same series.
- If they are different, the connector replaces them and starts a new series. The start time of the series is
  the record time.

The activity of an entity is the time of the latest record of its resource. Every record counts, also a record
that does not match the rule. For each resource in a batch, the connector reads the identity of each rule that has
entities. If an entity with this identity exists, its activity moves to the latest record time of the resource.
The activity never moves back. A resource without a known entity creates no entity. Thus, a pod that keeps
logging stays in the state, and a pod that stopped logging is the first to go.

A rule tracks at most `max_entities` entities. If the rule is at this limit, a new entity causes the eviction of
the entity of that rule that has the oldest activity. `max_state_bytes` limits an estimate of the size of the
serialized state, for all rules together. The estimate counts each key and value with its JSON escapes. If the
state is too large, the connector evicts the entities with the oldest activity, from any rule. The connector does
not keep an entity that alone is larger than `max_state_bytes`. In one batch, the connector updates the activity
before it adds new entities. Thus, the eviction for a new entity uses the latest activity.

### Emission

Each entity is one series: a gauge with the value `1` and the rule name as metric name. The resource has the
included resource attributes, and thus also the identity. If an included resource attribute is missing or empty,
the resource does not have it. The data point attributes are the facts. The data point time is the emission time.
The start time is the start of the series. The record time is never the data point time, because CloudWatch
rejects data points that are older than 14 days.

The connector emits in two ways:

- On change. If `emit_on_change` is `true`, a batch that adds an entity, or changes its facts or its included
  resource attributes, causes an immediate emission of these entities. Each entity is in the emission one time,
  with its final facts.
- On ticks. At each emit interval, the connector emits all the entities of the rules with that interval. If
  `emit_on_change` is `false`, the ticks also run.

The effective interval of a rule is its `emit_interval`, else the connector `emit_interval`. Rules with the same
effective interval share one timer. Each tick sends all their entities in one call to the next consumer. If these
rules have no entity, the tick sends nothing.

At startup, the connector does not emit. The first tick of each interval comes 10 seconds after the start, plus
a random delay. The random delay is less than the interval and less than 10 seconds. The 10 seconds give the
k8sattributes processor in the metrics pipeline time to fill its pod cache. Without them, a filter on its
attributes drops all the series of the first emission. The random delay spreads the ticks of agents that start
at the same time. After the first tick, a tick comes at each interval.

If the next consumer returns an error, the connector logs it, at most one time each minute. The error does not
go back to the logs pipeline, and the next ticks still run. At shutdown, the connector stops the ticks. It waits
for a tick in progress until the shutdown deadline. If the shutdown cancels a tick in progress, the connector logs
the error of the next consumer at the Debug level. This error does not count in `emit_errors`. With `storage`, the
connector writes the state before it waits for the tick.

### Persistence

If `storage` is set, the connector keeps its state in the storage extension. The state is one JSON record with the
key `state` and the version `1`. The record holds these values for each rule and entity: the identity, the
included resource attributes, and the facts. It also holds the start time of the series, the time of the latest
facts, and the activity.

At startup, the connector reads the record one time. After that, it only writes the record. The emission always
uses the state in memory. The connector writes the record at these times:

- Within about 1 second after a change. A change is a new entity, new facts, or an eviction. All the changes in
  that second go into one write.
- At most one time in 5 minutes, for entities whose activity or time of the latest facts moved, with no change.
- At shutdown, for a change or an activity that the connector did not write yet. A tick in progress does not
  delay this write.

At startup, the connector does not emit. The first tick sends the restored entities. The pods do not have to log
the startup line again. Their series keep their labels and their start times.

If the record is corrupt or has another version, the connector logs a warning and starts with an empty state. If
there is no usable record, the connector writes the current state about 1 second after the start. Thus, the
connector logs the warning only one time. If the read from the storage extension fails, the connector also logs a
warning and starts with an empty state. It does not replace the record until a change, because the record can
be valid.

If the storage extension does not exist or is not a storage extension, the configuration is wrong. The connector
then returns an error at startup, and the collector does not start. If the storage extension exists but cannot
give the connector its storage, the connector logs an error and starts without persistence. An example is a
`file_storage` extension that cannot open or create its file. The log has the error and the `storage` ID. The
connector then works the same as without `storage`. The state is in memory only and does not survive a restart.
The connector does not try the storage again during that run. If the storage opens again at the next start, the
connector restores the record from before the failure. The facts that the connector learned during the failure
are then lost. The connector emits the restored, older facts until the next matching record of each entity, for
example the next vLLM engine startup line.

The connector fits the restored state to the current configuration:

- The connector removes the entities of a rule that is not configured. A renamed rule is a new rule.
- If the `identity` keys of a rule changed, the connector removes the entities of that rule.
- The facts keep only the configured attributes, in the configured order. The connector cuts each value to
  `max_value_length`.
- If an attribute without `default_value` and without `optional` has no value, the connector removes the entity.
  A new attribute stays absent until the next matching record of the entity. This also applies with
  `default_value`, because the default value can be wrong for the entity.
- The resource keeps only the `include_resource_attributes`, in that order. The connector cuts each value to
  `max_value_length`.
- If an identity value is longer than `max_value_length` or is not valid UTF-8, the connector removes the entity.
  No record can give its identity again.
- `max_entities` and `max_state_bytes` apply. The connector keeps the entities with the latest activity.

After it reads a record, the connector writes one Info log with the number of entities that it kept, removed, and
changed. If it changed or removed an entity, it writes the cleaned state about 1 second after the start.

`max_state_bytes` also bounds the record. The size estimate of the state is never less than the size of its
entities in the record. The limit of the record is `max_state_bytes` plus a fixed margin for the record frame and
the rule names. Thus, the connector can always write a state that fits in `max_state_bytes`.

### Telemetry

The connector reports these metrics in the collector telemetry. The meter name is
`github.com/aws/amazon-cloudwatch-agent/connector/signaltoinfometricconnector`. The `rule` attribute is the rule
name.

| Metric                                                        | Type    | Attributes        | Description                                                                                        |
|---------------------------------------------------------------|---------|-------------------|----------------------------------------------------------------------------------------------------|
| `otelcol_connector_signal_to_info_metric_records_matched`     | Counter | `rule`            | Records that pass the `body_contains` check and the conditions of the rule.                        |
| `otelcol_connector_signal_to_info_metric_records_not_used`    | Counter | `rule`, `reason`  | Records that pass the `body_contains` check but that the rule does not use. Refer to the next list. |
| `otelcol_connector_signal_to_info_metric_values_truncated`    | Counter | `rule`            | Values cut to `max_value_length`. An identity or resource value counts one time for each resource. |
| `otelcol_connector_signal_to_info_metric_entities_evicted`    | Counter | `rule`, `reason`  | Entities that the state removed or did not store. `rule` is the rule of the entity.                |
| `otelcol_connector_signal_to_info_metric_datapoints_emitted`  | Counter | `rule`, `trigger` | Data points that the next consumer accepted. `trigger` is `change` or `interval`.                  |
| `otelcol_connector_signal_to_info_metric_emit_errors`         | Counter | `rule`            | Failed calls to the next consumer. A call counts one time for each rule in it.                     |
| `otelcol_connector_signal_to_info_metric_entities`            | Gauge   | `rule`            | Entities in the state.                                                                             |
| `otelcol_connector_signal_to_info_metric_state_bytes`         | Gauge   | none              | Size estimate of the serialized state, in bytes.                                                   |
| `otelcol_connector_signal_to_info_metric_state_saves`         | Counter | none              | Writes of the state record to the storage extension.                                               |
| `otelcol_connector_signal_to_info_metric_state_save_errors`   | Counter | none              | Failed writes of the state record. The snapshot, the encoding, or the storage write failed.        |
| `otelcol_connector_signal_to_info_metric_state_save_rejected` | Counter | none              | State records that the connector did not write, because they are larger than the record limit.     |
| `otelcol_connector_signal_to_info_metric_state_saved_bytes`   | Gauge   | none              | Size of the state record that the connector last read or wrote, in bytes.                          |

The `state_saves`, `state_save_errors`, `state_save_rejected`, and `state_saved_bytes` metrics exist only with a
storage that the connector opened at startup.

The `reason` values of `records_not_used`:

- `identity_missing`: an identity attribute of the resource is missing or empty.
- `required_missing`: an attribute without `default_value` and without `optional` has no value. An evaluation
  error of its expression also counts as no value, except with `error_mode: propagate`.
- `evaluation_error`: with `error_mode: propagate`, an evaluation error of a condition or of a required attribute
  stopped the batch at this record.

The `reason` values of `entities_evicted`:

- `max_entities`: a new entity of the same rule needed room.
- `max_state_bytes`: the state was larger than `max_state_bytes`.
- `too_large`: the entity alone is larger than `max_state_bytes`. The connector removed it, or did not store it.

Records that fail the `body_contains` check or the conditions change no metric. The only exception is the
`evaluation_error` reason of `records_not_used`.

### Logs

The connector writes these logs at most one time each minute. In `suppressed_since_last_log`, each log has the
number of events that the connector did not log since the previous log. For the eviction logs, an event is one
evicted entity.

- Warn, one for `max_entities` and one for `max_state_bytes`: the connector evicted the least active entities to
  stay within this limit. If the evicted entities were still active, the limit is too small. For example, the
  entities of deleted pods are not active, and their eviction is normal. The log has the `limit`, the `rule` of the
  first evicted entity, and the number of `evicted` entities.
- Info, one for each rule: a matching record was not used because a required attribute has no value. A change of
  the log format often causes this. If the collector telemetry level is `none`, this log is the only visible sign.
  `error_mode: silent` turns this log off.
- Warn: the next consumer returned an error.

If `storage` is set, the connector also logs a failed write of the state at the Error level. It logs a record
that is too large at the Warn level. Each of the two logs comes at most one time each minute. If the connector
cannot open the storage at startup, it logs this one time at the Error level. Refer to [Persistence](#persistence).

The other logs of the connector about single records and batches are at the Debug level. With
`error_mode: ignore`, OTTL logs each evaluation error of a condition at the Warn level, with no rate limit.

[OTTL]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/README.md
[OTTL functions]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/ottlfuncs/README.md#converters
