# SSM Documents and Parameters Cleanup Tool

This tool cleans up test SSM documents and parameters that may be left behind from CloudWatch Agent testing.

## What it cleans

### SSM Documents
- Documents with prefixes:
  - `Test-AmazonCloudWatch-ManageAgent-` (used by ssm_document tests)

### SSM Parameters
- Parameters with exact names:
  - `agentConfig1` (used by ssm_document tests)
  - `agentConfig2` (used by ssm_document tests)

**Note**: These patterns have been verified to only match test resources and will not affect production SSM documents or parameters.

## Usage

```bash
# Dry run (default) - shows what would be deleted without actually deleting
go run ./clean_ssm_documents.go

# Actually delete resources
go run ./clean_ssm_documents.go --dry-run=false

# Enable verbose logging to see all API calls
go run ./clean_ssm_documents.go --verbose

# Combine flags
go run ./clean_ssm_documents.go --dry-run=false --verbose
```

## Configuration

The tool is configured to:
- Clean resources older than 1 day
- Use 10 concurrent workers for processing
- Run in dry-run mode by default for safety

## Integration

This tool is integrated into the GitHub Actions workflow `clean-aws-resources.yml` and runs daily across multiple AWS regions to clean up test resources automatically.