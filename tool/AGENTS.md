# tool/ — CLI Utilities and Helpers

## What This Is

Shared utility packages used by the various CLI binaries in `cmd/`. These are the building blocks for config download, translation, and the setup wizard.

## Key Packages

| Package | Purpose |
|---------|---------|
| `wizard/` | Interactive config wizard — walks users through generating a JSON config file |
| `downloader/` | Downloads config from SSM Parameter Store or S3 |
| `translator/` | Translation helper utilities (distinct from the main `translator/` package) |
| `paths/` | OS-specific file paths (config dir, log dir, binary dir) |
| `runtime/` | Runtime environment detection |
| `processors/` | Config post-processing utilities |
| `data/` | Data types for config wizard |
| `stdin/` | Interactive stdin input handling |
| `cmdwrapper/` | Command execution wrappers |
| `clean/` | Config cleanup utilities |
| `testutil/` | Test helpers |

## Key Patterns

- `paths/` is platform-aware — Linux uses `/opt/aws/amazon-cloudwatch-agent/`, Windows uses `C:\ProgramData\Amazon\AmazonCloudWatchAgent\`, macOS uses `/opt/aws/amazon-cloudwatch-agent/`.
- `downloader/` supports multiple config sources: SSM Parameter Store, S3, local file. The source is determined by the `config-downloader` binary's flags.
- `wizard/` generates a complete JSON config by asking the user questions about their environment.
