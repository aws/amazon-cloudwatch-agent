# Add Cloud Metadata Placeholder Substitution

## Problem

CloudWatch Agent configuration files require instance-specific values (instance ID, region, account ID, etc.) that vary across deployments. Currently, users must manually configure these values or use separate placeholder systems for AWS (`${aws:...}`) and Azure (`${azure:...}`), leading to:

- Configuration duplication across cloud providers
- Manual updates when moving between clouds
- No unified way to reference cloud metadata

## Solution

Introduce universal `{cloud:...}` placeholders that work across all cloud providers, while maintaining backward compatibility with existing `${aws:...}` and `${azure:...}` placeholders. The system automatically resolves placeholders using the cloud metadata provider at config translation time.

## Architecture

```
┌─────────────────────────────────────┐
│      Config Translation             │
│   (placeholderUtil.go)              │
│                                     │
│  1. Detect placeholder type:        │
│     • {cloud:...}                   │
│     • ${aws:...}                    │
│     • ${azure:...}                  │
│                                     │
│  2. Get metadata provider           │
│     (cloudmetadata singleton)       │
│                                     │
│  3. Resolve placeholders:           │
│     • Exact match                   │
│     • Embedded in strings           │
│     • Multiple per string           │
└─────────────────┬───────────────────┘
                  │
                  ▼
┌─────────────────────────────────────┐
│    Cloud Metadata Provider          │
│      (global singleton)             │
│                                     │
│  • GetInstanceID()                  │
│  • GetRegion()                      │
│  • GetAccountID()                   │
│  • GetInstanceType()                │
│  • GetPrivateIP()                   │
│  • GetAvailabilityZone()            │
│  • GetImageID()                     │
└─────────────────┬───────────────────┘
                  │
        ┌─────────┴─────────┐
        │                   │
        ▼                   ▼
┌──────────────┐    ┌──────────────┐
│ AWS Provider │    │Azure Provider│
│              │    │              │
│ EC2 IMDS     │    │Azure IMDS    │
└──────────────┘    └──────────────┘
```

**Key Design Decisions:**

| Decision | Rationale |
|----------|-----------|
| Universal `{cloud:...}` syntax | Works across all cloud providers |
| Backward compatible | Existing `${aws:...}` and `${azure:...}` still work |
| Embedded placeholder support | Allows `"/logs/{cloud:InstanceId}/app"` |
| Graceful fallback | Falls back to legacy providers if new provider unavailable |
| Config translation time | Resolved once during config load, not at runtime |

## Changes

### Placeholder Resolution (`translator/translate/util/placeholderUtil.go`)

**New Functions:**
- `ResolveCloudMetadataPlaceholders()` - Resolves all placeholder types
- `resolveCloudPlaceholder()` - Handles `{cloud:...}` syntax
- `resolveAzurePlaceholder()` - Handles `${azure:...}` syntax (enhanced)
- `resolveAWSPlaceholder()` - Handles `${aws:...}` syntax (enhanced)

**Features:**
- Exact match replacement: `{"instance": "{cloud:InstanceId}"}`
- Embedded placeholders: `{"path": "/logs/{cloud:InstanceId}/app"}`
- Multiple placeholders: `{"name": "{cloud:Region}-{cloud:InstanceType}"}`
- Mixed cloud types: `{"aws": "${aws:InstanceId}", "azure": "${azure:VmId}"}`

### Supported Placeholders

#### Universal Cloud Placeholders

```
{cloud:InstanceId}         - Instance/VM ID
{cloud:Region}             - Region/Location
{cloud:AccountId}          - Account/Subscription ID
{cloud:InstanceType}       - Instance/VM size
{cloud:PrivateIp}          - Private IP address
{cloud:AvailabilityZone}   - Availability zone (AWS only)
{cloud:ImageId}            - AMI/Image ID
```

#### Azure-Specific Placeholders (Enhanced)

```
${azure:InstanceId}        - VM ID
${azure:InstanceType}      - VM size
${azure:Region}            - Location
${azure:AccountId}         - Subscription ID
${azure:ResourceGroupName} - Resource group
${azure:VmScaleSetName}    - VMSS name
${azure:PrivateIp}         - Private IP
```

#### AWS-Specific Placeholders (Existing)

```
${aws:InstanceId}          - EC2 instance ID
${aws:InstanceType}        - EC2 instance type
${aws:Region}              - AWS region
${aws:AvailabilityZone}    - Availability zone
${aws:ImageId}             - AMI ID
```

### Integration with Cloud Metadata Provider

The placeholder resolution system integrates with the cloud metadata provider (introduced in the IMDS PR):

1. **Initialization**: Provider initialized at agent startup
2. **Detection**: Cloud provider auto-detected (AWS, Azure, or Unknown)
3. **Resolution**: Placeholders resolved using provider's metadata
4. **Fallback**: Falls back to legacy code if provider unavailable

### Example Configurations

**Before (AWS-specific):**
```json
{
  "logs": {
    "logs_collected": {
      "files": {
        "collect_list": [
          {
            "file_path": "/var/log/app.log",
            "log_group_name": "/aws/ec2/${aws:InstanceId}",
            "log_stream_name": "${aws:InstanceId}-app"
          }
        ]
      }
    }
  }
}
```

**After (Cloud-agnostic):**
```json
{
  "logs": {
    "logs_collected": {
      "files": {
        "collect_list": [
          {
            "file_path": "/var/log/app.log",
            "log_group_name": "/aws/ec2/{cloud:InstanceId}",
            "log_stream_name": "{cloud:InstanceId}-app"
          }
        ]
      }
    }
  }
}
```

**Mixed placeholders (Azure-specific + universal):**
```json
{
  "metrics": {
    "append_dimensions": {
      "InstanceId": "{cloud:InstanceId}",
      "Region": "{cloud:Region}",
      "ResourceGroup": "${azure:ResourceGroupName}",
      "Environment": "production"
    }
  }
}
```

## Testing

### Unit Tests

**New Tests** (`translator/translate/util/placeholderUtil_test.go`):
- `TestResolveCloudMetadataPlaceholders_*` - Universal placeholder resolution
- `TestResolveAzureMetadataPlaceholders_EmbeddedPlaceholders` - Azure embedded placeholders
- `TestResolveAWSMetadataPlaceholders_EmbeddedPlaceholders` - AWS embedded placeholders
- Edge cases: nil inputs, non-map inputs, empty values

**Coverage:**
- 30+ new tests for placeholder resolution
- Embedded placeholder scenarios
- Mixed placeholder types
- Fallback behavior

### Manual Verification

**AWS EC2 (us-west-2):**
- ✅ `{cloud:InstanceId}` resolves to EC2 instance ID
- ✅ `{cloud:Region}` resolves to `us-west-2`
- ✅ Embedded placeholders work: `/logs/{cloud:InstanceId}/app`

**Azure VM (eastus2):**
- ✅ `{cloud:InstanceId}` resolves to Azure VM ID
- ✅ `{cloud:Region}` resolves to `eastus2`
- ✅ `${azure:ResourceGroupName}` resolves correctly
- ✅ Mixed placeholders work

**Local (no cloud):**
- ✅ Graceful fallback to defaults
- ✅ Agent continues without errors

## Backward Compatibility

✅ **Existing configurations unchanged**
- `${aws:...}` placeholders continue to work
- `${azure:...}` placeholders continue to work
- No breaking changes to config format

✅ **Graceful degradation**
- If cloud metadata provider unavailable, falls back to legacy code
- Agent continues to run with reduced functionality

✅ **No changes to existing behavior**
- AWS metadata fetching unchanged
- Azure metadata fetching unchanged
- Only adds new `{cloud:...}` syntax

## Migration Path

Users can migrate gradually:

1. **Phase 1**: Use existing `${aws:...}` or `${azure:...}` (no changes needed)
2. **Phase 2**: Adopt `{cloud:...}` for new configs (cloud-agnostic)
3. **Phase 3**: Migrate existing configs to `{cloud:...}` (optional)

No forced migration required - all syntaxes work simultaneously.

## Dependencies

This PR depends on the cloud metadata provider infrastructure introduced in the Azure IMDS support PR. It should be merged after that PR is approved.

## Verification Commands

```bash
# Build
make build

# Run tests
go test ./translator/translate/util/... -v -run "TestResolve.*Placeholders"

# Lint
make lint
```

## Related PRs

- Azure IMDS Support PR (prerequisite)
