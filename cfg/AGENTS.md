# cfg/ — AWS Configuration and Credentials

## What This Is

Handles AWS credential resolution, shared config loading, and environment-based configuration. This is the foundation for all AWS API calls the agent makes.

## Subdirectories

| Directory | Purpose |
|-----------|---------|
| `aws/` | Credential chain, STS assume-role, shared config parsing, SDK logging |
| `commonconfig/` | Common config TOML parsing (proxy, credentials file path, shared config) |
| `envconfig/` | Environment variable-based configuration |

## Credential Chain

`aws/credentials.go` implements a custom credential chain:

1. Root credentials from chain (env vars → shared credentials file → EC2 instance profile)
2. Optional STS AssumeRole on top of root credentials
3. Refreshable shared credentials provider for long-running agents

The chain is overridable via `OverwriteCredentialsChain()` for testing.

## Key Patterns

- `CredentialConfig` struct is the main entry point — call `.Credentials()` to get a configured `client.ConfigProvider`.
- STS credentials include fallback endpoints and regions for partition-aware assume-role (handles GovCloud, China, ISO partitions).
- `Refreshable_shared_credentials_provider` re-reads the credentials file on expiry — important for agents running on EC2 with rotating credentials.
- SDK logging is controlled via `aws_sdk_logging.go` and respects the agent's log level.

## What Must Never Happen

- Don't hardcode AWS partitions — the agent runs in commercial, GovCloud (`us-gov-*`), China (`cn-*`), ISO (`us-iso-*`), and ISOB (`us-isob-*`) regions. STS endpoints differ per partition.
- Don't remove the refreshable credentials provider — long-running agents need credential rotation.
- Don't bypass the credential chain — all AWS clients must go through `CredentialConfig.Credentials()`.
- Don't assume IMDS is always available — on-premises and some container environments don't have it.
