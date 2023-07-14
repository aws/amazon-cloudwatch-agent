## Private Integration Tests
The `test` module is meant to serve as a place for integration tests that cannot be placed in the external `amazon-cloudwatch-agent-test` repo.
These follow the pattern established by the external test repo and import dependencies from it to reuse as much as possible. Therefore, there are
a few requirements that are needed before running the tests.

### Base Requirements
- GoLang 1.18+
- A built and installed version of the agent from this repo

### Trace
The trace integration tests. Verifies that traces using both X-Ray and OTLP SDKs can be received and sent to X-Ray using the agent.

#### Additional Requirements
- IAM role used must include `AWSXRayDaemonWriteAccess` policy or cover all permissions provided by it