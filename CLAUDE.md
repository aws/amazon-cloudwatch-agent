# CloudWatch Agent

Go agent that collects metrics, logs, and traces for Amazon CloudWatch.

## Log group encryption

`logs.logs_collected.files.collect_list` and `windows_events.collect_list` accept `kms_key_id`. When the agent creates the log group, that value is sent as `CreateLogGroup.KmsKeyId`. Use a key alias (`alias/key-name`) or a key ARN. An existing log group is left unchanged.

Two collect entries for the same log group must use the same `kms_key_id`.

## Commands

```sh
go test ./plugins/outputs/cloudwatchlogs/internal/pusher ./translator/translate/logs/logs_collected/files/collect_list ./plugins/inputs/logfile
```

## Conventions

- Match existing package structure and log prefixes (`I!`, `E!`, `D!`, `W!`).
- Keep PRs focused; do not reformat unrelated files.
