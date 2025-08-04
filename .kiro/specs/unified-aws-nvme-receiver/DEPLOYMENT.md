# AWS NVMe Receiver Deployment and Rollback Procedures

## Overview

This document provides detailed deployment and rollback procedures for the unified AWS NVMe receiver (`awsnvmereceiver`). These procedures are designed to ensure zero-downtime migration from existing receivers while providing safe rollback options.

## Deployment Strategy

### Deployment Phases

1. **Pre-Deployment Validation** (30 minutes)
2. **Staged Deployment** (1-2 hours)
3. **Production Deployment** (2-4 hours)
4. **Post-Deployment Validation** (1 hour)

### Deployment Models

#### Model 1: Blue-Green Deployment (Recommended)
- Deploy to new instances alongside existing ones
- Validate functionality before switching traffic
- Provides safest rollback option

#### Model 2: Rolling Deployment
- Update instances one by one
- Suitable for large fleets
- Gradual risk exposure

#### Model 3: Canary Deployment
- Deploy to small subset of instances first
- Monitor for issues before full rollout
- Best for risk-averse environments

## Pre-Deployment Validation

### Environment Assessment Checklist

```bash
#!/bin/bash
# pre_deployment_assessment.sh

echo "=== Pre-Deployment Assessment ==="

# 1. Check current CloudWatch Agent version
echo "1. Current CloudWatch Agent version:"
sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -m ec2 -a query | grep -i version

# 2. Check current receiver configuration
echo "2. Current NVMe receiver configuration:"
sudo cat /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json | \
  jq '.metrics.metrics_collected.diskio // "No diskio configuration found"'

# 3. Check available NVMe devices
echo "3. Available NVMe devices:"
ls -la /dev/nvme* 2>/dev/null || echo "No NVMe devices found"

# 4. Check device types (if possible)
echo "4. Device type detection test:"
for device in /dev/nvme*n*; do
  if [ -e "$device" ]; then
    echo "Device: $device"
    sudo nvme id-ctrl "$device" 2>/dev/null | grep -E "mn|sn" || echo "  Unable to query device"
  fi
done

# 5. Check current metrics in CloudWatch
echo "5. Current metrics availability:"
INSTANCE_ID=$(curl -s http://169.254.169.254/latest/meta-data/instance-id 2>/dev/null || echo "unknown")
echo "Instance ID: $INSTANCE_ID"

# Check for EBS metrics
aws cloudwatch list-metrics \
  --namespace "CWAgent" \
  --metric-name "diskio_ebs_total_read_ops" \
  --query 'length(Metrics)' 2>/dev/null || echo "No EBS metrics found"

# Check for Instance Store metrics
aws cloudwatch list-metrics \
  --namespace "EC2InstanceStoreMetrics" \
  --metric-name "diskio_instance_store_total_read_ops" \
  --query 'length(Metrics)' 2>/dev/null || echo "No Instance Store metrics found"

# 6. Check permissions
echo "6. Permission checks:"
echo "  CloudWatch Agent user: $(ps aux | grep amazon-cloudwatch-agent | grep -v grep | awk '{print $1}' | head -1)"
echo "  Device permissions:"
ls -la /dev/nvme* 2>/dev/null | head -5

# 7. Check system resources
echo "7. System resources:"
echo "  CPU usage: $(top -bn1 | grep "Cpu(s)" | awk '{print $2}' | cut -d'%' -f1)"
echo "  Memory usage: $(free | grep Mem | awk '{printf "%.1f%%", $3/$2 * 100.0}')"
echo "  Disk usage: $(df -h / | awk 'NR==2{printf "%s", $5}')"

echo "=== Pre-Deployment Assessment Complete ==="
```

### Validation Criteria

Before proceeding with deployment, ensure:

- [ ] CloudWatch Agent is running and healthy
- [ ] Current NVMe metrics are being collected successfully
- [ ] All target devices are accessible
- [ ] System resources are within acceptable limits (CPU < 80%, Memory < 80%)
- [ ] No critical alerts or issues in monitoring systems
- [ ] Backup of current configuration is created

## Deployment Procedures

### Blue-Green Deployment (Recommended)

#### Phase 1: Prepare Green Environment

```bash
#!/bin/bash
# blue_green_deploy_phase1.sh

echo "=== Blue-Green Deployment Phase 1: Prepare Green Environment ==="

# 1. Create backup of current configuration
BACKUP_FILE="/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json.backup.$(date +%Y%m%d_%H%M%S)"
sudo cp /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json "$BACKUP_FILE"
echo "Configuration backed up to: $BACKUP_FILE"

# 2. Download new CloudWatch Agent version
echo "Downloading new CloudWatch Agent..."
cd /tmp
wget https://s3.amazonaws.com/amazoncloudwatch-agent/amazon_linux/amd64/latest/amazon-cloudwatch-agent.rpm
if [ $? -ne 0 ]; then
  echo "ERROR: Failed to download CloudWatch Agent"
  exit 1
fi

# 3. Validate package integrity
echo "Validating package integrity..."
rpm -K amazon-cloudwatch-agent.rpm
if [ $? -ne 0 ]; then
  echo "ERROR: Package integrity check failed"
  exit 1
fi

# 4. Create test configuration for validation
echo "Creating test configuration..."
cat > /tmp/test-config.json << 'EOF'
{
  "agent": {
    "metrics_collection_interval": 60,
    "logfile": "/opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log"
  },
  "metrics": {
    "namespace": "TestNamespace",
    "metrics_collected": {
      "diskio": {
        "resources": ["*"],
        "measurement": [
          "diskio_ebs_total_read_ops",
          "diskio_instance_store_total_read_ops"
        ]
      }
    }
  }
}
EOF

echo "=== Phase 1 Complete ==="
```

#### Phase 2: Install and Test Green Environment

```bash
#!/bin/bash
# blue_green_deploy_phase2.sh

echo "=== Blue-Green Deployment Phase 2: Install and Test ==="

# 1. Stop current agent (Blue environment continues on other instances)
echo "Stopping current CloudWatch Agent..."
sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -m ec2 -a stop

# 2. Install new version
echo "Installing new CloudWatch Agent version..."
sudo rpm -U /tmp/amazon-cloudwatch-agent.rpm
if [ $? -ne 0 ]; then
  echo "ERROR: Failed to install new version"
  # Rollback: restart old agent
  sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl \
    -m ec2 -c file:/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json -a start
  exit 1
fi

# 3. Test with existing configuration
echo "Testing with existing configuration..."
sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent \
  --config-file-path /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json \
  --dry-run
if [ $? -ne 0 ]; then
  echo "ERROR: Configuration validation failed"
  exit 1
fi

# 4. Start agent with existing configuration
echo "Starting agent with existing configuration..."
sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl \
  -m ec2 -c file:/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json -a start

# 5. Wait for startup and check status
sleep 30
sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -m ec2 -a query
if [ $? -ne 0 ]; then
  echo "ERROR: Agent failed to start properly"
  exit 1
fi

# 6. Check logs for errors
echo "Checking logs for errors..."
sudo tail -50 /opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log | grep -i error
if [ $? -eq 0 ]; then
  echo "WARNING: Errors found in logs, review required"
fi

# 7. Verify device detection
echo "Verifying device detection..."
sudo grep -i "awsnvmereceiver" /opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log | tail -5

echo "=== Phase 2 Complete ==="
```

#### Phase 3: Validate Green Environment

```bash
#!/bin/bash
# blue_green_deploy_phase3.sh

echo "=== Blue-Green Deployment Phase 3: Validate Green Environment ==="

# 1. Wait for metrics to be collected
echo "Waiting for metrics collection (5 minutes)..."
sleep 300

# 2. Check for new metrics in CloudWatch
INSTANCE_ID=$(curl -s http://169.254.169.254/latest/meta-data/instance-id)
echo "Checking metrics for instance: $INSTANCE_ID"

# Check EBS metrics
echo "Checking EBS metrics..."
aws cloudwatch get-metric-statistics \
  --namespace "EC2InstanceStoreMetrics" \
  --metric-name "diskio_ebs_total_read_ops" \
  --dimensions Name=instance_id,Value=$INSTANCE_ID Name=device_type,Value=ebs \
  --start-time $(date -u -d '10 minutes ago' +%Y-%m-%dT%H:%M:%S) \
  --end-time $(date -u +%Y-%m-%dT%H:%M:%S) \
  --period 300 \
  --statistics Sum \
  --query 'Datapoints[0].Sum'

# Check Instance Store metrics (if applicable)
echo "Checking Instance Store metrics..."
aws cloudwatch get-metric-statistics \
  --namespace "EC2InstanceStoreMetrics" \
  --metric-name "diskio_instance_store_total_read_ops" \
  --dimensions Name=instance_id,Value=$INSTANCE_ID Name=device_type,Value=instance_store \
  --start-time $(date -u -d '10 minutes ago' +%Y-%m-%dT%H:%M:%S) \
  --end-time $(date -u +%Y-%m-%dT%H:%M:%S) \
  --period 300 \
  --statistics Sum \
  --query 'Datapoints[0].Sum'

# 3. Verify resource attributes
echo "Verifying resource attributes..."
aws cloudwatch list-metrics \
  --namespace "EC2InstanceStoreMetrics" \
  --metric-name "diskio_ebs_total_read_ops" \
  --query 'Metrics[0].Dimensions'

# 4. Performance validation
echo "Performance validation..."
CPU_USAGE=$(top -bn1 | grep "Cpu(s)" | awk '{print $2}' | cut -d'%' -f1 | cut -d'u' -f1)
MEMORY_USAGE=$(free | grep Mem | awk '{printf "%.1f", $3/$2 * 100.0}')

echo "CPU Usage: ${CPU_USAGE}%"
echo "Memory Usage: ${MEMORY_USAGE}%"

if (( $(echo "$CPU_USAGE > 5.0" | bc -l) )); then
  echo "WARNING: High CPU usage detected"
fi

if (( $(echo "$MEMORY_USAGE > 90.0" | bc -l) )); then
  echo "WARNING: High memory usage detected"
fi

echo "=== Phase 3 Complete ==="
```

### Rolling Deployment

#### Rolling Deployment Script

```bash
#!/bin/bash
# rolling_deployment.sh

# Configuration
INSTANCE_LIST_FILE="/tmp/instances.txt"  # One instance ID per line
BATCH_SIZE=5
WAIT_BETWEEN_BATCHES=300  # 5 minutes

echo "=== Rolling Deployment Started ==="

# Validate instance list
if [ ! -f "$INSTANCE_LIST_FILE" ]; then
  echo "ERROR: Instance list file not found: $INSTANCE_LIST_FILE"
  exit 1
fi

TOTAL_INSTANCES=$(wc -l < "$INSTANCE_LIST_FILE")
echo "Total instances to update: $TOTAL_INSTANCES"
echo "Batch size: $BATCH_SIZE"

# Process instances in batches
BATCH_NUM=1
while IFS= read -r instance_id; do
  echo "Processing instance: $instance_id (Batch $BATCH_NUM)"
  
  # Deploy to instance using AWS Systems Manager
  aws ssm send-command \
    --instance-ids "$instance_id" \
    --document-name "AWS-RunShellScript" \
    --parameters 'commands=[
      "#!/bin/bash",
      "# Stop current agent",
      "sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -m ec2 -a stop",
      "# Backup configuration", 
      "sudo cp /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json.backup.$(date +%Y%m%d_%H%M%S)",
      "# Download and install new version",
      "cd /tmp && wget https://s3.amazonaws.com/amazoncloudwatch-agent/amazon_linux/amd64/latest/amazon-cloudwatch-agent.rpm",
      "sudo rpm -U /tmp/amazon-cloudwatch-agent.rpm",
      "# Start agent with existing configuration",
      "sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -m ec2 -c file:/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json -a start",
      "# Verify status",
      "sleep 30",
      "sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -m ec2 -a query"
    ]' \
    --comment "Deploy unified NVMe receiver - Batch $BATCH_NUM"
  
  # Check if we've reached batch size
  if (( BATCH_NUM % BATCH_SIZE == 0 )); then
    echo "Batch $BATCH_NUM complete. Waiting $WAIT_BETWEEN_BATCHES seconds..."
    sleep $WAIT_BETWEEN_BATCHES
    
    # Validate batch success before continuing
    echo "Validating batch $BATCH_NUM..."
    # Add validation logic here
  fi
  
  ((BATCH_NUM++))
  
done < "$INSTANCE_LIST_FILE"

echo "=== Rolling Deployment Complete ==="
```

### Canary Deployment

#### Canary Deployment Script

```bash
#!/bin/bash
# canary_deployment.sh

# Configuration
CANARY_PERCENTAGE=5  # Start with 5% of instances
CANARY_DURATION=3600  # Monitor for 1 hour
FULL_ROLLOUT_DELAY=1800  # Wait 30 minutes between phases

echo "=== Canary Deployment Started ==="

# Phase 1: Deploy to canary instances
echo "Phase 1: Deploying to $CANARY_PERCENTAGE% of instances..."

# Get list of instances (example using Auto Scaling Group)
ASG_NAME="your-asg-name"
aws autoscaling describe-auto-scaling-groups \
  --auto-scaling-group-names "$ASG_NAME" \
  --query 'AutoScalingGroups[0].Instances[].InstanceId' \
  --output text > /tmp/all_instances.txt

TOTAL_INSTANCES=$(wc -w < /tmp/all_instances.txt)
CANARY_COUNT=$(( TOTAL_INSTANCES * CANARY_PERCENTAGE / 100 ))
if [ $CANARY_COUNT -lt 1 ]; then
  CANARY_COUNT=1
fi

echo "Total instances: $TOTAL_INSTANCES"
echo "Canary instances: $CANARY_COUNT"

# Select canary instances (first N instances)
head -n $CANARY_COUNT /tmp/all_instances.txt > /tmp/canary_instances.txt

# Deploy to canary instances
while IFS= read -r instance_id; do
  echo "Deploying to canary instance: $instance_id"
  
  aws ssm send-command \
    --instance-ids "$instance_id" \
    --document-name "AWS-RunShellScript" \
    --parameters 'commands=[
      "#!/bin/bash",
      "sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -m ec2 -a stop",
      "sudo cp /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json.backup.$(date +%Y%m%d_%H%M%S)",
      "cd /tmp && wget https://s3.amazonaws.com/amazoncloudwatch-agent/amazon_linux/amd64/latest/amazon-cloudwatch-agent.rpm",
      "sudo rpm -U /tmp/amazon-cloudwatch-agent.rpm",
      "sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -m ec2 -c file:/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json -a start"
    ]' \
    --comment "Canary deployment - unified NVMe receiver"
    
done < /tmp/canary_instances.txt

# Phase 2: Monitor canary instances
echo "Phase 2: Monitoring canary instances for $CANARY_DURATION seconds..."
sleep $CANARY_DURATION

# Validate canary success
echo "Validating canary deployment..."
CANARY_SUCCESS=true

while IFS= read -r instance_id; do
  # Check CloudWatch metrics for this instance
  METRIC_COUNT=$(aws cloudwatch list-metrics \
    --namespace "EC2InstanceStoreMetrics" \
    --dimensions Name=instance_id,Value=$instance_id \
    --query 'length(Metrics)')
  
  if [ "$METRIC_COUNT" -eq 0 ]; then
    echo "ERROR: No metrics found for canary instance $instance_id"
    CANARY_SUCCESS=false
  fi
done < /tmp/canary_instances.txt

# Phase 3: Full rollout or rollback
if [ "$CANARY_SUCCESS" = true ]; then
  echo "Phase 3: Canary successful, proceeding with full rollout..."
  sleep $FULL_ROLLOUT_DELAY
  
  # Deploy to remaining instances
  tail -n +$(( CANARY_COUNT + 1 )) /tmp/all_instances.txt > /tmp/remaining_instances.txt
  
  while IFS= read -r instance_id; do
    echo "Deploying to instance: $instance_id"
    # Same deployment commands as canary
  done < /tmp/remaining_instances.txt
  
else
  echo "Phase 3: Canary failed, initiating rollback..."
  # Rollback canary instances
  while IFS= read -r instance_id; do
    echo "Rolling back canary instance: $instance_id"
    # Rollback commands here
  done < /tmp/canary_instances.txt
fi

echo "=== Canary Deployment Complete ==="
```

## Rollback Procedures

### Immediate Rollback (Emergency)

```bash
#!/bin/bash
# emergency_rollback.sh

echo "=== EMERGENCY ROLLBACK INITIATED ==="

# 1. Stop current agent immediately
echo "Stopping CloudWatch Agent..."
sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -m ec2 -a stop

# 2. Find most recent backup
BACKUP_FILE=$(ls -t /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json.backup.* 2>/dev/null | head -1)

if [ -z "$BACKUP_FILE" ]; then
  echo "ERROR: No backup configuration found!"
  echo "Manual intervention required."
  exit 1
fi

echo "Found backup: $BACKUP_FILE"

# 3. Restore backup configuration
sudo cp "$BACKUP_FILE" /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json
echo "Configuration restored from backup"

# 4. Downgrade CloudWatch Agent if needed
echo "Checking if downgrade is needed..."
# This would need to be customized based on your environment
# Example: if you have the previous version package available
if [ -f "/tmp/previous-amazon-cloudwatch-agent.rpm" ]; then
  echo "Downgrading CloudWatch Agent..."
  sudo rpm -U --oldpackage /tmp/previous-amazon-cloudwatch-agent.rpm
fi

# 5. Start agent with restored configuration
echo "Starting CloudWatch Agent with restored configuration..."
sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl \
  -m ec2 -c file:/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json -a start

# 6. Verify rollback success
sleep 30
sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -m ec2 -a query

if [ $? -eq 0 ]; then
  echo "=== EMERGENCY ROLLBACK SUCCESSFUL ==="
else
  echo "=== EMERGENCY ROLLBACK FAILED - MANUAL INTERVENTION REQUIRED ==="
  exit 1
fi
```

### Planned Rollback

```bash
#!/bin/bash
# planned_rollback.sh

echo "=== Planned Rollback Started ==="

# 1. Validate current state
echo "Validating current state..."
sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -m ec2 -a query

# 2. Stop current agent gracefully
echo "Stopping CloudWatch Agent gracefully..."
sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -m ec2 -a stop

# Wait for graceful shutdown
sleep 10

# 3. List available backups
echo "Available configuration backups:"
ls -la /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json.backup.*

# 4. Select backup (most recent by default)
BACKUP_FILE=$(ls -t /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json.backup.* | head -1)
echo "Selected backup: $BACKUP_FILE"

# 5. Validate backup configuration
echo "Validating backup configuration..."
sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent \
  --config-file-path "$BACKUP_FILE" --dry-run

if [ $? -ne 0 ]; then
  echo "ERROR: Backup configuration is invalid"
  exit 1
fi

# 6. Restore configuration
sudo cp "$BACKUP_FILE" /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json
echo "Configuration restored"

# 7. Downgrade agent if necessary
read -p "Do you need to downgrade the CloudWatch Agent? (y/n): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
  echo "Please provide the path to the previous version package:"
  read -r PREVIOUS_PACKAGE
  
  if [ -f "$PREVIOUS_PACKAGE" ]; then
    sudo rpm -U --oldpackage "$PREVIOUS_PACKAGE"
    echo "Agent downgraded"
  else
    echo "ERROR: Previous package not found: $PREVIOUS_PACKAGE"
    exit 1
  fi
fi

# 8. Start agent with restored configuration
echo "Starting CloudWatch Agent..."
sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl \
  -m ec2 -c file:/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json -a start

# 9. Verify rollback
sleep 30
echo "Verifying rollback..."
sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -m ec2 -a query

# 10. Check metrics collection
echo "Waiting for metrics collection..."
sleep 300

INSTANCE_ID=$(curl -s http://169.254.169.254/latest/meta-data/instance-id)
aws cloudwatch list-metrics \
  --namespace "CWAgent" \
  --dimensions Name=InstanceId,Value=$INSTANCE_ID \
  --query 'length(Metrics)'

echo "=== Planned Rollback Complete ==="
```

### Fleet-Wide Rollback

```bash
#!/bin/bash
# fleet_rollback.sh

# Configuration
INSTANCE_LIST_FILE="/tmp/instances.txt"
BATCH_SIZE=10
WAIT_BETWEEN_BATCHES=60

echo "=== Fleet-Wide Rollback Started ==="

# Validate instance list
if [ ! -f "$INSTANCE_LIST_FILE" ]; then
  echo "ERROR: Instance list file not found: $INSTANCE_LIST_FILE"
  exit 1
fi

TOTAL_INSTANCES=$(wc -l < "$INSTANCE_LIST_FILE")
echo "Total instances to rollback: $TOTAL_INSTANCES"

# Process instances in batches
BATCH_NUM=1
while IFS= read -r instance_id; do
  echo "Rolling back instance: $instance_id (Batch $BATCH_NUM)"
  
  # Rollback instance using AWS Systems Manager
  aws ssm send-command \
    --instance-ids "$instance_id" \
    --document-name "AWS-RunShellScript" \
    --parameters 'commands=[
      "#!/bin/bash",
      "echo \"Starting rollback for $(hostname)\"",
      "# Stop current agent",
      "sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -m ec2 -a stop",
      "# Find most recent backup",
      "BACKUP_FILE=$(ls -t /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json.backup.* | head -1)",
      "if [ -z \"$BACKUP_FILE\" ]; then echo \"ERROR: No backup found\"; exit 1; fi",
      "echo \"Using backup: $BACKUP_FILE\"",
      "# Restore backup",
      "sudo cp \"$BACKUP_FILE\" /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json",
      "# Start agent",
      "sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -m ec2 -c file:/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json -a start",
      "# Verify",
      "sleep 30",
      "sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -m ec2 -a query"
    ]' \
    --comment "Fleet rollback - unified NVMe receiver"
  
  # Check if we've reached batch size
  if (( BATCH_NUM % BATCH_SIZE == 0 )); then
    echo "Batch $(( BATCH_NUM / BATCH_SIZE )) complete. Waiting $WAIT_BETWEEN_BATCHES seconds..."
    sleep $WAIT_BETWEEN_BATCHES
  fi
  
  ((BATCH_NUM++))
  
done < "$INSTANCE_LIST_FILE"

echo "=== Fleet-Wide Rollback Complete ==="
```

## Post-Deployment Validation

### Comprehensive Validation Script

```bash
#!/bin/bash
# post_deployment_validation.sh

echo "=== Post-Deployment Validation ==="

# 1. Agent Status Validation
echo "1. Validating CloudWatch Agent status..."
sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -m ec2 -a query
AGENT_STATUS=$?

if [ $AGENT_STATUS -ne 0 ]; then
  echo "ERROR: CloudWatch Agent is not running properly"
  exit 1
fi

# 2. Log Analysis
echo "2. Analyzing agent logs..."
LOG_FILE="/opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log"

# Check for errors
ERROR_COUNT=$(sudo grep -c -i error "$LOG_FILE" | tail -100)
echo "Recent error count: $ERROR_COUNT"

if [ "$ERROR_COUNT" -gt 10 ]; then
  echo "WARNING: High error count in logs"
  sudo grep -i error "$LOG_FILE" | tail -10
fi

# Check for successful receiver startup
sudo grep -i "awsnvmereceiver.*start" "$LOG_FILE" | tail -5

# 3. Device Detection Validation
echo "3. Validating device detection..."
sudo grep -i "device.*type.*detected" "$LOG_FILE" | tail -10

# 4. Metrics Validation
echo "4. Validating metrics in CloudWatch..."
INSTANCE_ID=$(curl -s http://169.254.169.254/latest/meta-data/instance-id)

# Wait for metrics to appear
echo "Waiting 5 minutes for metrics to appear..."
sleep 300

# Check EBS metrics
EBS_METRIC_COUNT=$(aws cloudwatch list-metrics \
  --namespace "EC2InstanceStoreMetrics" \
  --metric-name "diskio_ebs_total_read_ops" \
  --dimensions Name=instance_id,Value=$INSTANCE_ID \
  --query 'length(Metrics)')

echo "EBS metrics found: $EBS_METRIC_COUNT"

# Check Instance Store metrics
INSTANCE_STORE_METRIC_COUNT=$(aws cloudwatch list-metrics \
  --namespace "EC2InstanceStoreMetrics" \
  --metric-name "diskio_instance_store_total_read_ops" \
  --dimensions Name=instance_id,Value=$INSTANCE_ID \
  --query 'length(Metrics)')

echo "Instance Store metrics found: $INSTANCE_STORE_METRIC_COUNT"

# 5. Resource Attribute Validation
echo "5. Validating resource attributes..."
aws cloudwatch list-metrics \
  --namespace "EC2InstanceStoreMetrics" \
  --metric-name "diskio_ebs_total_read_ops" \
  --query 'Metrics[0].Dimensions'

# 6. Performance Validation
echo "6. Validating performance impact..."
CPU_USAGE=$(top -bn1 | grep "Cpu(s)" | awk '{print $2}' | cut -d'%' -f1 | cut -d'u' -f1)
MEMORY_USAGE=$(free | grep Mem | awk '{printf "%.1f", $3/$2 * 100.0}')

echo "Current CPU usage: ${CPU_USAGE}%"
echo "Current memory usage: ${MEMORY_USAGE}%"

# 7. Metric Value Validation
echo "7. Validating metric values..."
aws cloudwatch get-metric-statistics \
  --namespace "EC2InstanceStoreMetrics" \
  --metric-name "diskio_ebs_total_read_ops" \
  --dimensions Name=instance_id,Value=$INSTANCE_ID Name=device_type,Value=ebs \
  --start-time $(date -u -d '10 minutes ago' +%Y-%m-%dT%H:%M:%S) \
  --end-time $(date -u +%Y-%m-%dT%H:%M:%S) \
  --period 300 \
  --statistics Sum \
  --query 'Datapoints[0].Sum'

echo "=== Post-Deployment Validation Complete ==="

# Summary
echo "=== Validation Summary ==="
echo "Agent Status: $([ $AGENT_STATUS -eq 0 ] && echo 'PASS' || echo 'FAIL')"
echo "Log Errors: $([ $ERROR_COUNT -lt 10 ] && echo 'PASS' || echo 'WARNING')"
echo "EBS Metrics: $([ $EBS_METRIC_COUNT -gt 0 ] && echo 'PASS' || echo 'FAIL')"
echo "Instance Store Metrics: $([ $INSTANCE_STORE_METRIC_COUNT -gt 0 ] && echo 'PASS' || echo 'N/A')"
echo "Performance: $([ $(echo "$CPU_USAGE < 5.0" | bc -l) -eq 1 ] && echo 'PASS' || echo 'WARNING')"
```

## Monitoring and Alerting

### Key Metrics to Monitor

1. **Agent Health**
   - CloudWatch Agent process status
   - Log error rates
   - Memory and CPU usage

2. **Metric Collection**
   - Metric ingestion rates
   - Missing metric alerts
   - Dimension consistency

3. **Performance**
   - Scrape latency
   - Resource utilization
   - Error rates

### CloudWatch Alarms

```json
{
  "AlarmName": "NVMeReceiver-HighErrorRate",
  "AlarmDescription": "High error rate in NVMe receiver logs",
  "MetricName": "ErrorCount",
  "Namespace": "CWAgent",
  "Statistic": "Sum",
  "Period": 300,
  "EvaluationPeriods": 2,
  "Threshold": 10,
  "ComparisonOperator": "GreaterThanThreshold"
}
```

## Troubleshooting Guide

### Common Deployment Issues

1. **Agent fails to start**
   - Check configuration syntax
   - Verify permissions
   - Review log files

2. **No metrics appearing**
   - Verify device accessibility
   - Check IMDS connectivity
   - Validate configuration

3. **Performance degradation**
   - Monitor resource usage
   - Check for memory leaks
   - Validate device access patterns

### Recovery Procedures

1. **Configuration corruption**
   - Restore from backup
   - Validate configuration
   - Restart agent

2. **Permission issues**
   - Verify CAP_SYS_ADMIN
   - Check device permissions
   - Validate IAM roles

3. **Network connectivity**
   - Test IMDS access
   - Verify CloudWatch endpoints
   - Check security groups

## Conclusion

These deployment and rollback procedures provide comprehensive coverage for safely migrating to the unified AWS NVMe receiver. The procedures are designed to minimize risk and provide multiple safety nets for different deployment scenarios.

Key success factors:
- Thorough pre-deployment validation
- Staged deployment approach
- Comprehensive monitoring
- Quick rollback capabilities
- Post-deployment validation

Following these procedures ensures a smooth transition while maintaining system reliability and observability.