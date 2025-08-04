# AWS NVMe Receiver Migration Guide

## Overview

This document provides comprehensive migration guidance for transitioning from the existing `awsebsnvmereceiver` and `awsinstancestorenvmereceiver` to the unified `awsnvmereceiver`. The unified receiver consolidates both EBS and Instance Store NVMe device monitoring into a single component while maintaining full backward compatibility.

## Migration Summary

| Aspect | Before | After | Compatibility |
|--------|--------|-------|---------------|
| **Receiver Type** | `awsebsnvmereceiver` + `awsinstancestorenvmereceiver` | `awsnvmereceiver` | ✅ Full backward compatibility |
| **Configuration** | Separate configurations | Unified `diskio` configuration | ✅ Existing configs work unchanged |
| **Metric Names** | Same prefixes | Same prefixes maintained | ✅ No breaking changes |
| **Resource Attributes** | Different attribute names | Unified attribute names | ⚠️ Minor changes (see details) |
| **Device Discovery** | Separate logic | Unified auto-detection | ✅ Enhanced functionality |

## Pre-Migration Checklist

### 1. Environment Assessment
- [ ] Identify current receiver usage (`awsebsnvmereceiver` and/or `awsinstancestorenvmereceiver`)
- [ ] Document existing configuration parameters
- [ ] Inventory monitored devices (EBS vs Instance Store)
- [ ] Review current metric collection patterns
- [ ] Identify any custom dashboards or alarms using NVMe metrics

### 2. Backup Current Configuration
```bash
# Backup current CloudWatch Agent configuration
sudo cp /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json \
       /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json.backup.$(date +%Y%m%d_%H%M%S)
```

### 3. Verify Prerequisites
- [ ] CloudWatch Agent version supports unified receiver
- [ ] Required permissions (CAP_SYS_ADMIN) are available
- [ ] NVMe devices are accessible under `/dev/nvme*`
- [ ] Instance Metadata Service (IMDS) is accessible

## Migration Scenarios

### Scenario 1: EBS-Only Environment (Most Common)

#### Current Configuration
```json
{
  "metrics": {
    "namespace": "CWAgent",
    "metrics_collected": {
      "diskio": {
        "resources": ["*"],
        "measurement": [
          "diskio_ebs_total_read_ops",
          "diskio_ebs_total_write_ops",
          "diskio_ebs_total_read_bytes",
          "diskio_ebs_total_write_bytes"
        ]
      }
    }
  }
}
```

#### Migration Steps
1. **No configuration changes required** - existing configuration works as-is
2. Update CloudWatch Agent to version with unified receiver
3. Restart CloudWatch Agent
4. Verify metrics continue to flow with same names and values

#### Validation
```bash
# Check that EBS metrics are still being collected
aws logs filter-log-events \
  --log-group-name /aws/amazoncloudwatch-agent \
  --filter-pattern "diskio_ebs_total_read_ops"
```

### Scenario 2: Instance Store-Only Environment

#### Current Configuration
```json
{
  "metrics": {
    "namespace": "EC2InstanceStoreMetrics", 
    "metrics_collected": {
      "diskio": {
        "resources": ["*"],
        "measurement": [
          "diskio_instance_store_total_read_ops",
          "diskio_instance_store_total_write_ops"
        ]
      }
    }
  }
}
```

#### Migration Steps
1. **No configuration changes required** - existing configuration works as-is
2. Update CloudWatch Agent to version with unified receiver
3. Restart CloudWatch Agent
4. Verify Instance Store metrics continue to flow

### Scenario 3: Mixed Environment (EBS + Instance Store)

#### Current Configuration (Separate Receivers)
```json
{
  "metrics": {
    "namespace": "EC2InstanceStoreMetrics",
    "metrics_collected": {
      "diskio": {
        "resources": ["*"],
        "measurement": [
          "diskio_ebs_total_read_ops",
          "diskio_ebs_total_write_ops", 
          "diskio_instance_store_total_read_ops",
          "diskio_instance_store_total_write_ops"
        ]
      }
    }
  }
}
```

#### Migration Steps
1. **No configuration changes required** - unified receiver handles both device types automatically
2. Update CloudWatch Agent to version with unified receiver
3. Restart CloudWatch Agent
4. Verify both EBS and Instance Store metrics are collected

#### Enhanced Functionality
The unified receiver provides automatic device type detection, so you get:
- Automatic discovery of both EBS and Instance Store devices
- Proper metric prefixing based on detected device type
- Unified resource attributes for better filtering

## Configuration Migration Details

### Supported Configuration Parameters

All existing configuration parameters are fully supported:

```json
{
  "metrics": {
    "namespace": "EC2InstanceStoreMetrics",
    "metrics_collection_interval": 60,
    "metrics_collected": {
      "diskio": {
        "resources": ["*"],                    // ✅ Fully supported
        "measurement": [...],                  // ✅ All existing metrics supported
        "totalinclude": ["*"],                // ✅ Supported (if used)
        "report_deltas": true                 // ✅ Supported (if used)
      }
    }
  }
}
```

### Device Discovery Configuration

| Configuration | Behavior | Compatibility |
|---------------|----------|---------------|
| `"resources": ["*"]` | Auto-discover all NVMe devices | ✅ Enhanced - detects both types |
| `"resources": ["/dev/nvme0n1"]` | Monitor specific device | ✅ Same behavior |
| `"resources": ["/dev/nvme*"]` | Pattern matching | ✅ Same behavior |
| Empty resources | Default auto-discovery | ✅ Same behavior |

## Resource Attributes Migration

### Attribute Name Changes

The unified receiver standardizes resource attribute names:

| Old Receiver | Old Attribute | New Attribute | Migration Impact |
|--------------|---------------|---------------|------------------|
| `awsebsnvmereceiver` | `VolumeId` | `instance_id` | ⚠️ **Breaking Change** |
| `awsinstancestorenvmereceiver` | `InstanceId` | `instance_id` | ✅ No change |
| `awsinstancestorenvmereceiver` | `Device` | `device` | ✅ No change |
| `awsinstancestorenvmereceiver` | `SerialNumber` | `serial_number` | ✅ No change |

### New Unified Attributes

All metrics now include these standardized attributes:

```json
{
  "instance_id": "i-1234567890abcdef0",     // EC2 instance ID
  "device_type": "ebs",                     // "ebs" or "instance_store"  
  "device": "/dev/nvme0n1",                 // Device path
  "serial_number": "vol-12345678"           // Device serial number
}
```

### Impact on Dashboards and Alarms

#### EBS Metrics - Action Required ⚠️
If you have dashboards or alarms filtering on the `VolumeId` dimension:

**Before:**
```json
{
  "MetricName": "diskio_ebs_total_read_ops",
  "Dimensions": [
    {"Name": "VolumeId", "Value": "vol-12345678"}
  ]
}
```

**After:**
```json
{
  "MetricName": "diskio_ebs_total_read_ops", 
  "Dimensions": [
    {"Name": "instance_id", "Value": "i-1234567890abcdef0"},
    {"Name": "device_type", "Value": "ebs"},
    {"Name": "serial_number", "Value": "vol-12345678"}
  ]
}
```

#### Instance Store Metrics - No Action Required ✅
Instance Store metrics maintain the same dimension names.

## Metric Compatibility Matrix

### EBS Metrics
| Metric Name | Old Receiver | New Receiver | Unit Change | Notes |
|-------------|--------------|--------------|-------------|-------|
| `diskio_ebs_total_read_ops` | ✅ | ✅ | None | Fully compatible |
| `diskio_ebs_total_write_ops` | ✅ | ✅ | None | Fully compatible |
| `diskio_ebs_total_read_bytes` | ✅ | ✅ | None | Fully compatible |
| `diskio_ebs_total_write_bytes` | ✅ | ✅ | None | Fully compatible |
| `diskio_ebs_total_read_time` | ✅ | ✅ | `us` → `ns` | ⚠️ **Unit change** |
| `diskio_ebs_total_write_time` | ✅ | ✅ | `us` → `ns` | ⚠️ **Unit change** |
| `diskio_ebs_volume_performance_exceeded_iops` | ✅ | ✅ | `us` → `ns` | ⚠️ **Unit change** |
| `diskio_ebs_volume_performance_exceeded_tp` | ✅ | ✅ | `us` → `ns` | ⚠️ **Unit change** |
| `diskio_ebs_ec2_instance_performance_exceeded_iops` | ✅ | ✅ | `us` → `ns` | ⚠️ **Unit change** |
| `diskio_ebs_ec2_instance_performance_exceeded_tp` | ✅ | ✅ | `us` → `ns` | ⚠️ **Unit change** |
| `diskio_ebs_volume_queue_length` | ✅ | ✅ | None | Fully compatible |

### Instance Store Metrics
| Metric Name | Old Receiver | New Receiver | Unit Change | Notes |
|-------------|--------------|--------------|-------------|-------|
| `diskio_instance_store_total_read_ops` | ✅ | ✅ | None | Fully compatible |
| `diskio_instance_store_total_write_ops` | ✅ | ✅ | None | Fully compatible |
| `diskio_instance_store_total_read_bytes` | ✅ | ✅ | None | Fully compatible |
| `diskio_instance_store_total_write_bytes` | ✅ | ✅ | None | Fully compatible |
| `diskio_instance_store_total_read_time` | ✅ | ✅ | None | Fully compatible (already `ns`) |
| `diskio_instance_store_total_write_time` | ✅ | ✅ | None | Fully compatible (already `ns`) |
| `diskio_instance_store_volume_performance_exceeded_iops` | ✅ | ✅ | None | Fully compatible (already `ns`) |
| `diskio_instance_store_volume_performance_exceeded_tp` | ✅ | ✅ | None | Fully compatible (already `ns`) |
| `diskio_instance_store_volume_queue_length` | ✅ | ✅ | None | Fully compatible |

### Unit Standardization Impact ⚠️

The unified receiver standardizes time units to nanoseconds for consistency:

**EBS Time Metrics Unit Change:**
- **Before:** microseconds (`us`)
- **After:** nanoseconds (`ns`) 
- **Conversion:** Values will be 1000x larger (1 µs = 1000 ns)

**Affected Dashboards/Alarms:**
If you have dashboards or alarms using EBS time metrics, you may need to:
1. Update display units in dashboards
2. Adjust alarm thresholds (multiply by 1000)
3. Update any calculations that assume microsecond units

## Step-by-Step Migration Procedure

### Phase 1: Preparation (1-2 hours)

1. **Inventory Current Setup**
   ```bash
   # Check current receiver configuration
   sudo cat /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json | \
     jq '.metrics.metrics_collected.diskio'
   
   # Check current CloudWatch Agent version
   sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl \
     -m ec2 -c file:/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json \
     -a query
   ```

2. **Document Current Metrics**
   ```bash
   # List current NVMe-related metrics in CloudWatch
   aws cloudwatch list-metrics \
     --namespace "CWAgent" \
     --metric-name "diskio_ebs_total_read_ops"
   
   aws cloudwatch list-metrics \
     --namespace "EC2InstanceStoreMetrics" \
     --metric-name "diskio_instance_store_total_read_ops"
   ```

3. **Backup Configuration**
   ```bash
   # Create timestamped backup
   sudo cp /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json \
          /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json.backup.$(date +%Y%m%d_%H%M%S)
   ```

### Phase 2: Update CloudWatch Agent (30 minutes)

1. **Stop Current Agent**
   ```bash
   sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl \
     -m ec2 -c file:/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json \
     -a stop
   ```

2. **Update CloudWatch Agent**
   ```bash
   # Download and install latest version
   wget https://s3.amazonaws.com/amazoncloudwatch-agent/amazon_linux/amd64/latest/amazon-cloudwatch-agent.rpm
   sudo rpm -U ./amazon-cloudwatch-agent.rpm
   ```

3. **Verify New Receiver is Available**
   ```bash
   # Check that awsnvmereceiver is available
   sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent \
     --config-file-path /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json \
     --dry-run 2>&1 | grep -i nvme
   ```

### Phase 3: Start Unified Receiver (15 minutes)

1. **Start Agent with Existing Configuration**
   ```bash
   sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl \
     -m ec2 -c file:/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json \
     -a start
   ```

2. **Verify Agent Status**
   ```bash
   sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl \
     -m ec2 -c file:/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json \
     -a query
   ```

3. **Check Logs for Successful Startup**
   ```bash
   sudo tail -f /opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log
   ```

### Phase 4: Validation (30 minutes)

1. **Verify Device Detection**
   ```bash
   # Check logs for device type detection
   sudo grep -i "device.*type.*detected" /opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log
   ```

2. **Verify Metric Collection**
   ```bash
   # Wait 2-3 minutes for first metrics, then check CloudWatch
   aws cloudwatch get-metric-statistics \
     --namespace "EC2InstanceStoreMetrics" \
     --metric-name "diskio_ebs_total_read_ops" \
     --dimensions Name=instance_id,Value=$(curl -s http://169.254.169.254/latest/meta-data/instance-id) \
     --start-time $(date -u -d '5 minutes ago' +%Y-%m-%dT%H:%M:%S) \
     --end-time $(date -u +%Y-%m-%dT%H:%M:%S) \
     --period 300 \
     --statistics Sum
   ```

3. **Compare Metric Values**
   ```bash
   # Compare current values with historical data to ensure consistency
   # Values should be similar (accounting for unit changes in time metrics)
   ```

## Rollback Procedure

If issues are encountered during migration, follow this rollback procedure:

### Immediate Rollback (5 minutes)

1. **Stop Current Agent**
   ```bash
   sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl \
     -m ec2 -a stop
   ```

2. **Restore Previous Configuration**
   ```bash
   # Find the backup file
   ls -la /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json.backup.*
   
   # Restore the most recent backup
   sudo cp /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json.backup.YYYYMMDD_HHMMSS \
          /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json
   ```

3. **Downgrade CloudWatch Agent (if needed)**
   ```bash
   # If you need to downgrade to previous version
   sudo yum downgrade amazon-cloudwatch-agent
   # or
   sudo apt-get install amazon-cloudwatch-agent=<previous-version>
   ```

4. **Restart with Previous Configuration**
   ```bash
   sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl \
     -m ec2 -c file:/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json \
     -a start
   ```

### Verify Rollback Success

```bash
# Verify agent is running with old receivers
sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl \
  -m ec2 -a query

# Check logs for successful startup
sudo tail -f /opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log
```

## Post-Migration Tasks

### 1. Update Dashboards and Alarms

For EBS metrics, update any dashboards or alarms that reference the old `VolumeId` dimension:

**CloudWatch Console:**
1. Navigate to CloudWatch → Dashboards
2. Edit widgets using EBS NVMe metrics
3. Update dimension filters from `VolumeId` to `serial_number`
4. Add `device_type=ebs` filter for clarity

**Terraform/CloudFormation:**
```hcl
# Before
resource "aws_cloudwatch_metric_alarm" "ebs_read_ops" {
  alarm_name          = "high-ebs-read-ops"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "diskio_ebs_total_read_ops"
  namespace           = "CWAgent"
  period              = "300"
  statistic           = "Sum"
  threshold           = "1000"
  
  dimensions = {
    VolumeId = "vol-12345678"
  }
}

# After  
resource "aws_cloudwatch_metric_alarm" "ebs_read_ops" {
  alarm_name          = "high-ebs-read-ops"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "diskio_ebs_total_read_ops"
  namespace           = "EC2InstanceStoreMetrics"
  period              = "300"
  statistic           = "Sum"
  threshold           = "1000"
  
  dimensions = {
    instance_id    = "i-1234567890abcdef0"
    device_type    = "ebs"
    serial_number  = "vol-12345678"
  }
}
```

### 2. Update Time-Based Calculations

For EBS time metrics that changed from microseconds to nanoseconds:

**Before (microseconds):**
```sql
-- CloudWatch Insights query
SELECT AVG(diskio_ebs_total_read_time) / AVG(diskio_ebs_total_read_ops) as avg_read_latency_us
FROM SCHEMA("CWAgent", VolumeId)
```

**After (nanoseconds):**
```sql
-- CloudWatch Insights query  
SELECT AVG(diskio_ebs_total_read_time) / AVG(diskio_ebs_total_read_ops) / 1000 as avg_read_latency_us
FROM SCHEMA("EC2InstanceStoreMetrics", instance_id, device_type, serial_number)
WHERE device_type = 'ebs'
```

### 3. Validate Enhanced Functionality

Test the new unified capabilities:

```bash
# Verify both device types are detected in mixed environments
aws cloudwatch list-metrics \
  --namespace "EC2InstanceStoreMetrics" \
  --dimensions Name=device_type,Value=ebs

aws cloudwatch list-metrics \
  --namespace "EC2InstanceStoreMetrics" \
  --dimensions Name=device_type,Value=instance_store
```

## Troubleshooting

### Common Issues and Solutions

#### Issue: No metrics after migration
**Symptoms:** CloudWatch Agent starts successfully but no NVMe metrics appear

**Diagnosis:**
```bash
# Check agent logs for errors
sudo grep -i error /opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log

# Check device permissions
ls -la /dev/nvme*

# Verify IMDS access
curl -s http://169.254.169.254/latest/meta-data/instance-id
```

**Solutions:**
1. Ensure CloudWatch Agent has CAP_SYS_ADMIN capability
2. Verify NVMe devices are accessible
3. Check IMDS is not blocked by security groups/NACLs

#### Issue: Different metric values after migration
**Symptoms:** Metric values are significantly different from before migration

**Diagnosis:**
```bash
# Check for unit conversion issues (EBS time metrics)
# Old values in microseconds, new values in nanoseconds (1000x larger)

# Verify device type detection
sudo grep -i "device.*type" /opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log
```

**Solutions:**
1. For EBS time metrics: Values should be 1000x larger (µs → ns conversion)
2. Verify device type detection is working correctly
3. Check that measurement list includes desired metrics

#### Issue: Missing dimensions in CloudWatch
**Symptoms:** Metrics appear but with different or missing dimensions

**Diagnosis:**
```bash
# Compare old vs new metric dimensions
aws cloudwatch list-metrics --namespace "CWAgent" --metric-name "diskio_ebs_total_read_ops"
aws cloudwatch list-metrics --namespace "EC2InstanceStoreMetrics" --metric-name "diskio_ebs_total_read_ops"
```

**Solutions:**
1. Update dashboards/alarms to use new dimension names
2. For EBS: Change `VolumeId` to `serial_number`
3. Add `device_type` dimension for filtering

#### Issue: Agent fails to start after migration
**Symptoms:** CloudWatch Agent fails to start with configuration errors

**Diagnosis:**
```bash
# Check configuration validation
sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent \
  --config-file-path /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json \
  --dry-run
```

**Solutions:**
1. Verify configuration syntax is valid JSON
2. Check that all required fields are present
3. Ensure device paths in configuration are valid
4. Rollback to previous configuration if needed

## Testing and Validation Scripts

### Pre-Migration Validation Script

```bash
#!/bin/bash
# pre_migration_validation.sh

echo "=== Pre-Migration Validation ==="

# Check current agent status
echo "1. Checking CloudWatch Agent status..."
sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -m ec2 -a query

# Check current configuration
echo "2. Current diskio configuration:"
sudo cat /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json | jq '.metrics.metrics_collected.diskio'

# Check available NVMe devices
echo "3. Available NVMe devices:"
ls -la /dev/nvme* 2>/dev/null || echo "No NVMe devices found"

# Check current metrics in CloudWatch
echo "4. Current EBS metrics:"
aws cloudwatch list-metrics --namespace "CWAgent" --metric-name "diskio_ebs_total_read_ops" --query 'Metrics[0].Dimensions'

echo "5. Current Instance Store metrics:"
aws cloudwatch list-metrics --namespace "EC2InstanceStoreMetrics" --metric-name "diskio_instance_store_total_read_ops" --query 'Metrics[0].Dimensions'

echo "=== Pre-Migration Validation Complete ==="
```

### Post-Migration Validation Script

```bash
#!/bin/bash
# post_migration_validation.sh

echo "=== Post-Migration Validation ==="

# Check agent status
echo "1. Checking CloudWatch Agent status..."
sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -m ec2 -a query

# Check for errors in logs
echo "2. Checking for errors in agent logs..."
sudo grep -i error /opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log | tail -10

# Check device type detection
echo "3. Checking device type detection..."
sudo grep -i "device.*type.*detected" /opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log | tail -5

# Verify new metrics structure
echo "4. New EBS metrics structure:"
aws cloudwatch list-metrics --namespace "EC2InstanceStoreMetrics" --metric-name "diskio_ebs_total_read_ops" --query 'Metrics[0].Dimensions'

echo "5. New Instance Store metrics structure:"
aws cloudwatch list-metrics --namespace "EC2InstanceStoreMetrics" --metric-name "diskio_instance_store_total_read_ops" --query 'Metrics[0].Dimensions'

# Check metric values
INSTANCE_ID=$(curl -s http://169.254.169.254/latest/meta-data/instance-id)
echo "6. Recent metric values for instance $INSTANCE_ID:"

aws cloudwatch get-metric-statistics \
  --namespace "EC2InstanceStoreMetrics" \
  --metric-name "diskio_ebs_total_read_ops" \
  --dimensions Name=instance_id,Value=$INSTANCE_ID Name=device_type,Value=ebs \
  --start-time $(date -u -d '10 minutes ago' +%Y-%m-%dT%H:%M:%S) \
  --end-time $(date -u +%Y-%m-%dT%H:%M:%S) \
  --period 300 \
  --statistics Sum \
  --query 'Datapoints[0].Sum'

echo "=== Post-Migration Validation Complete ==="
```

## Support and Escalation

### Internal Support Contacts
- **Primary:** AWS CloudWatch Agent Team
- **Secondary:** EC2 Instance Store Team (for Instance Store specific issues)
- **Escalation:** AWS Support (Enterprise/Business plans)

### Useful Log Locations
- **Agent Logs:** `/opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log`
- **Configuration:** `/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json`
- **System Logs:** `/var/log/messages` or `/var/log/syslog`

### Key Log Messages to Monitor
- `"awsnvmereceiver started successfully"`
- `"device type detected: ebs|instance_store"`
- `"failed to detect device type"` (indicates issues)
- `"ioctl operation failed"` (permission/device issues)

## Conclusion

The migration to the unified `awsnvmereceiver` provides enhanced functionality while maintaining backward compatibility for most use cases. The main considerations are:

1. **✅ Configuration:** Existing configurations work without changes
2. **✅ Metric Names:** All metric names remain the same
3. **⚠️ Resource Attributes:** EBS metrics change from `VolumeId` to `instance_id`/`serial_number`
4. **⚠️ Units:** EBS time metrics change from microseconds to nanoseconds
5. **✅ Enhanced Features:** Automatic device type detection and unified monitoring

Following this migration guide ensures a smooth transition with minimal disruption to existing monitoring workflows.