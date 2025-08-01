# Requirements Document

## Introduction

This feature implements a unified AWS NVMe receiver (`awsnvmereceiver`) for the Amazon CloudWatch Agent that consolidates metric collection for both EC2 EBS NVMe devices and Instance Store NVMe devices into a single OpenTelemetry Collector receiver. The receiver will replace both the existing `awsebsnvmereceiver` and `awsinstancestorenvmereceiver` with a unified implementation that uses device type detection to route logic appropriately, ensuring compatibility with the provided JSON configuration and exact Instance Store log page structure including histograms.

## Requirements

### Requirement 1

**User Story:** As a CloudWatch Agent user, I want a unified receiver that automatically detects and monitors both EBS and Instance Store NVMe devices, so that I can use a single configuration for all NVMe storage monitoring.

#### Acceptance Criteria

1. WHEN the receiver is configured THEN the system SHALL scan for all NVMe devices and automatically detect device types
2. WHEN a device is detected THEN the system SHALL determine if it's EBS or Instance Store based on device model names and magic number validation
3. WHEN EBS devices are found THEN the system SHALL use existing EBS log page parsing logic
4. WHEN Instance Store devices are found THEN the system SHALL validate magic number 0xEC2C0D7E and use Instance Store parsing logic
5. WHEN mixed device environments exist THEN the system SHALL handle both types simultaneously without conflicts

### Requirement 2

**User Story:** As a system administrator, I want to configure NVMe monitoring through the existing diskio section, so that I can maintain compatibility with current configurations.

#### Acceptance Criteria

1. WHEN the receiver is configured THEN the system SHALL use the existing `metrics.metrics_collected.diskio` configuration section
2. WHEN resources is set to ["*"] THEN the system SHALL automatically discover and monitor all NVMe devices regardless of type
3. WHEN specific device paths are provided THEN the system SHALL monitor only those devices and detect their types
4. WHEN measurement list includes both EBS and Instance Store metrics THEN the system SHALL emit only applicable metrics for each device type
5. WHEN namespace is set to "EC2InstanceStoreMetrics" THEN the system SHALL use this namespace for all metrics

### Requirement 3

**User Story:** As a performance analyst, I want to collect comprehensive metrics from both EBS and Instance Store devices with appropriate prefixes, so that I can distinguish between device types in my analysis.

#### Acceptance Criteria

1. WHEN EBS devices are monitored THEN the system SHALL emit metrics with "diskio_ebs_" prefix:
   - diskio_ebs_total_read_ops, diskio_ebs_total_write_ops
   - diskio_ebs_total_read_bytes, diskio_ebs_total_write_bytes  
   - diskio_ebs_total_read_time, diskio_ebs_total_write_time
   - diskio_ebs_volume_performance_exceeded_iops, diskio_ebs_volume_performance_exceeded_tp
   - diskio_ebs_ec2_instance_performance_exceeded_iops, diskio_ebs_ec2_instance_performance_exceeded_tp
   - diskio_ebs_volume_queue_length
2. WHEN Instance Store devices are monitored THEN the system SHALL emit metrics with "diskio_instance_store_" prefix:
   - diskio_instance_store_total_read_ops, diskio_instance_store_total_write_ops
   - diskio_instance_store_total_read_bytes, diskio_instance_store_total_write_bytes
   - diskio_instance_store_total_read_time, diskio_instance_store_total_write_time
   - diskio_instance_store_volume_performance_exceeded_iops, diskio_instance_store_volume_performance_exceeded_tp
   - diskio_instance_store_volume_queue_length
3. WHEN Instance Store histogram data is present THEN the system SHALL derive histogram metrics:
   - diskio_instance_store_read_latency_histogram, diskio_instance_store_write_latency_histogram
4. WHEN metrics are collected THEN the system SHALL handle cumulative counters and compute deltas appropriately

### Requirement 4

**User Story:** As a CloudWatch user, I want metrics to include device type information and standard dimensions, so that I can filter and aggregate data effectively.

#### Acceptance Criteria

1. WHEN metrics are emitted THEN the system SHALL include the following resource attributes:
   - instance_id (retrieved from EC2 Instance Metadata Service)
   - device_type ("ebs" or "instance_store")
   - device (device path like "/dev/nvme0n1")
   - serial_number (retrieved from NVMe device identification)
2. WHEN device type detection fails THEN the system SHALL log an error and skip that device
3. WHEN dimension retrieval fails THEN the system SHALL use placeholder values where possible and log warnings

### Requirement 5

**User Story:** As a developer, I want the receiver to reuse existing shared utilities and follow established patterns, so that maintenance is simplified and consistency is maintained.

#### Acceptance Criteria

1. WHEN implementing NVMe operations THEN the system SHALL extend the existing `internal/nvme` package with unified device detection
2. WHEN implementing the receiver interface THEN the system SHALL follow OpenTelemetry Collector receiver patterns
3. WHEN generating metadata THEN the system SHALL use mdatagen with comprehensive metric definitions
4. WHEN replacing awsebsnvmereceiver and awsinstancestorenvmereceiver THEN the system SHALL maintain backward compatibility for existing configurations
5. WHEN parsing Instance Store log pages THEN the system SHALL use the exact InstanceStoreMetrics struct with Magic, Reserved, ReadOps, WriteOps, ReadBytes, WriteBytes, TotalReadTime, TotalWriteTime, EBSIOPSExceeded (skip), EBSThroughputExceeded (skip), EC2IOPSExceeded, EC2ThroughputExceeded, QueueLength, NumHistograms, NumBins, IOSizeRange, Bounds, histogram bins, and ReservedArea fields
6. WHEN retrieving Instance Store metrics THEN the system SHALL use GetInstanceStoreMetrics function with 4KB buffer for log page 0xC0 and validate magic number 0xEC2C0D7E

### Requirement 6

**User Story:** As a system administrator, I want the receiver to handle errors gracefully and provide clear logging, so that I can troubleshoot issues effectively.

#### Acceptance Criteria

1. WHEN device access fails THEN the system SHALL log appropriate errors and continue with other devices
2. WHEN ioctl operations fail THEN the system SHALL log the failure reason with device context
3. WHEN log page parsing fails THEN the system SHALL log parsing errors and skip that device's metrics
4. WHEN device type detection fails THEN the system SHALL log detection failures and skip the device
5. WHEN counter overflow is detected THEN the system SHALL handle overflow gracefully and log warnings

### Requirement 7

**User Story:** As a performance-conscious administrator, I want the unified receiver to maintain optimal performance, so that monitoring doesn't impact application performance.

#### Acceptance Criteria

1. WHEN monitoring mixed device types THEN the system SHALL consume less than 1% CPU overhead per 60-second scrape cycle
2. WHEN processing up to 10 devices of mixed types THEN the system SHALL maintain performance requirements
3. WHEN performing device type detection THEN the system SHALL cache results to avoid repeated expensive operations
4. WHEN allocating memory for log pages THEN the system SHALL reuse buffers efficiently across device types

### Requirement 8

**User Story:** As a security-conscious administrator, I want the receiver to follow security best practices for both device types, so that it doesn't introduce vulnerabilities.

#### Acceptance Criteria

1. WHEN performing ioctl operations THEN the system SHALL require CAP_SYS_ADMIN capability
2. WHEN accessing device files THEN the system SHALL validate file paths to prevent directory traversal
3. WHEN processing log page data THEN the system SHALL validate data bounds for both EBS and Instance Store formats
4. WHEN the receiver fails to obtain required permissions THEN the system SHALL log appropriate errors and fail gracefully

### Requirement 9

**User Story:** As a quality assurance engineer, I want comprehensive test coverage for both device types, so that the unified receiver is reliable and maintainable.

#### Acceptance Criteria

1. WHEN unit tests are written THEN the system SHALL achieve greater than 90% code coverage
2. WHEN integration tests are created THEN the system SHALL validate metrics for both EBS and Instance Store devices
3. WHEN edge cases are tested THEN the system SHALL cover mixed device scenarios, detection failures, and parsing errors
4. WHEN performance tests are run THEN the system SHALL validate resource usage with mixed device types

### Requirement 10

**User Story:** As a CloudWatch Agent user, I want the unified receiver to work seamlessly in the existing pipeline, so that I can upgrade without configuration changes.

#### Acceptance Criteria

1. WHEN the receiver is deployed THEN the system SHALL integrate into the existing `metrics/hostDeltaMetrics` pipeline
2. WHEN replacing awsebsnvmereceiver THEN the system SHALL handle the same configuration parameters
3. WHEN processing metrics THEN the system SHALL work with existing processors (cumulativetodelta, ec2tagger, awsentity)
4. WHEN exporting metrics THEN the system SHALL work with the existing awscloudwatch exporter
5. WHEN the receiver encounters unsupported platforms THEN the system SHALL disable itself gracefully without affecting other receivers