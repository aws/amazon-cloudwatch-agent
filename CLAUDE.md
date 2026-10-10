# CloudWatch Agent

`timestamp_format` `%s` reads a Unix epoch timestamp from the log line. `%s.%f` and `%s%f` are seconds with a fractional part.

On the file tailer, an integer is seconds, milliseconds, microseconds, or nanoseconds based on its magnitude. A decimal point is always seconds plus a fraction. A JSON object uses the first usable `timestamp`, `time`, or `@timestamp` field, so an earlier number in the same object is not the event time.

The OpenTelemetry filelog receiver maps `%s` to stanza epoch layout `s` and `%s.%f` to `s.ns`. The layout value stored for the file tailer is `epoch`, which `time.Parse` cannot read. Call `ParseEpoch`.
