# CloudWatch Agent

Go agent that collects metrics, logs, and traces for Amazon CloudWatch.

## Config schema

`translator/config/schema.json` is the JSON Schema for the agent config. It is embedded by `translator/config` and copied into packages as `amazon-cloudwatch-agent-schema.json`.

The published URL is `translator/config.SchemaURL` and the schema `$id`. Keep those two the same (#1219).

## Commands

```sh
go test ./translator/config
```

## Conventions

- Match existing package structure.
- Keep PRs focused; do not reformat unrelated files.
