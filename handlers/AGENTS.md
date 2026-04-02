# handlers/ — AWS SDK Request Handlers

## What This Is

Custom AWS SDK v1 request handlers that inject headers and compress request bodies. These are attached to AWS SDK clients via `request.NamedHandler`.

## Files

- `customheader.go` — Injects custom HTTP headers into AWS API requests. Used for agent health reporting (the `agenthealth` extension marshals stats into a header value via `NewDynamicCustomHeaderHandler`).
- `compress.go` — Gzip compression for request bodies (CloudWatch PutMetricData, PutLogEvents).

## Key Pattern

These handlers use the AWS SDK v1 `request.NamedHandler` interface — they're added to the SDK's `Send` handler chain. They run on every AWS API call made through the configured client.

## What Must Never Happen

- Don't add handlers that modify request bodies after signing — SigV4 will reject the request. Compression handlers must run BEFORE the Sign handler in the chain.
- Don't add blocking handlers — they run synchronously on every API call.
- Don't add handlers that allocate large buffers — they run on every request and can cause memory pressure under high throughput.
