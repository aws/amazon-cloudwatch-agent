# CloudWatch Agent

Go agent that collects metrics, logs, and traces for Amazon CloudWatch.

## Version strings

S3, SSM, ECR, and Docker Hub names append a build id (`1.300064.1b1344`). Amazon Linux RPMs omit it and add a dist tag on the release (`1.300064.1-1.amzn2023`, `1.300064.1-1.amzn2`). The build id is not a different source release (#2049).

Compare `internal/version.UpstreamRelease`. Both forms above are upstream `1.300064.1`.

## Commands

```sh
go test ./internal/version
```

## Conventions

- Match existing package structure.
- Keep PRs focused; do not reformat unrelated files.
