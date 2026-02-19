# Cloud Auth Extension

The `cloudauth` extension provides OIDC token management for authenticating the
CloudWatch Agent to AWS from non-AWS environments (e.g., Azure VMs). It
auto-detects the cloud provider, fetches OIDC tokens, and writes them to a file
for the credential chain to use with `AssumeRoleWithWebIdentity`.

## How It Works

1. Extension detects the cloud provider (or reads a user-managed token file)
2. Fetches an OIDC token and writes it to disk
3. Sets `AWS_WEB_IDENTITY_TOKEN_FILE` env var pointing to the token file
4. The credential chain in `cfg/aws/credentials.go` detects the env var and
   uses each exporter's `credentials.role_arn` to call `AssumeRoleWithWebIdentity`

Each role used must have an IAM trust policy allowing the OIDC provider.

## Supported Providers

- **Azure** — Uses Azure Instance Metadata Service (IMDS) managed identity tokens.
- **File** — Reads a user-managed token from disk (`token_file` config option).

## Configuration

Minimal — just enable `oidc_auth` and set `role_arn` at the credentials level:

```json
{
  "agent": {
    "region": "us-west-2",
    "credentials": {
      "role_arn": "arn:aws:iam::123456789012:role/MyOIDCRole",
      "oidc_auth": {}
    }
  }
}
```

With a user-managed token file (for on-prem):

```json
{
  "agent": {
    "credentials": {
      "role_arn": "arn:aws:iam::123456789012:role/MyOIDCRole",
      "oidc_auth": {
        "token_file": "/var/run/oidc/token"
      }
    }
  }
}
```

### `oidc_auth` Fields

| Field | Required | Description |
|-------|----------|-------------|
| `token_file` | No | Path to a user-managed OIDC token file. Skips auto-detection. |
| `sts_resource` | No | Audience/resource claim for the OIDC token request. |

The `role_arn` at the `credentials` level (or per-section `metrics`/`logs` credentials)
is the IAM role assumed via `AssumeRoleWithWebIdentity`. Each role must have a
trust policy allowing the OIDC identity provider.
