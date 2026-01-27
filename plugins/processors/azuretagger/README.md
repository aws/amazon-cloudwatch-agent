# Azure Tagger Processor

The Azure Tagger processor adds Azure VM metadata and tags as dimensions to metrics.
This is the Azure equivalent of the `ec2tagger` processor for AWS.

## Configuration

```yaml
processors:
  azuretagger:
    # Interval for refreshing Azure tags from IMDS
    # Set to 0 to disable periodic refresh (tags fetched once at startup)
    # Default: 0
    refresh_tags_interval: 5m

    # Azure metadata fields to add as dimensions
    # Supported: InstanceId, InstanceType, ImageId, VMScaleSetName,
    #            ResourceGroupName, SubscriptionId
    azure_metadata_tags:
      - InstanceId
      - InstanceType
      - VMScaleSetName

    # Azure VM tags to add as dimensions
    # Use ["*"] to include all tags
    azure_instance_tag_keys:
      - Environment
      - Team
```

## Behavior

- **Non-Azure environments**: The processor is automatically disabled when not running on Azure
- **Graceful degradation**: If IMDS is unavailable, the processor starts without metadata
- **Tag refresh**: Tags are fetched from Azure IMDS (no IAM required, unlike AWS ec2:DescribeTags)
- **Existing attributes**: Existing metric attributes are not overwritten

## Differences from ec2tagger

| Aspect | ec2tagger (AWS) | azuretagger (Azure) |
|--------|-----------------|---------------------|
| Tag source | EC2 API (DescribeTags) | IMDS (local) |
| IAM required | Yes | No |
| ASG equivalent | AutoScalingGroupName | VMScaleSetName |
| Account ID | AWS Account ID | Azure Subscription ID |

## Supported Dimensions

| Dimension | Description |
|-----------|-------------|
| InstanceId | Azure VM ID |
| InstanceType | Azure VM Size (e.g., Standard_D2s_v3) |
| ImageId | Azure VM ID (Azure doesn't have AMI equivalent) |
| VMScaleSetName | VM Scale Set name (empty if not in VMSS) |
| ResourceGroupName | Azure Resource Group |
| SubscriptionId | Azure Subscription ID |
