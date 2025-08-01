# Requirements Document

## Introduction

This feature adds a new receiver (awsinstancestorenvmereceiver) to the Amazon CloudWatch Agent for collecting enhanced NVMe metrics from EC2 Instance Store volumes. The receiver will use vendor-specific log pages to gather detailed performance metrics and emit them to CloudWatch, mirroring the existing awsebsnvmereceiver but targeting Instance Store devices specifically.

## Requirements

### Requirement 1

**User Story:** As a CloudWatch Agent user, I want to collect NVMe metrics from EC2 Instance Store volumes, so that I can monitor the performance and health of my instance storage.

#### Acceptance Criteria

1. WHEN the receiver is configured THEN the system SHALL scan for NVMe devices matching "Amazon EC2 NVMe Instance Storage" model name
2. WHEN a matching device is found THEN the system SHALL use NVME_IOCTL_ID_CTRL to verify device identity via model name in bytes 40-79
3. WHEN log page 0xC0 is retrieved THEN the system SHALL validate the magic number 0xEC2C0D7E to confirm the device is an Instance Store volume
4. WHEN device detection fails THEN the system SHALL log appropriate error messages and continue with other devices
5. IF no matching devices are found THEN the system SHALL log a warning and continue operation without errors

### Requirement 2

**User Story:** As a system administrator, I want to configure which Instance Store devices to monitor, so that I can control resource usage and focus on specific volumes.

#### Acceptance Criteria

1. WHEN the receiver is configured THEN the system SHALL support a "resources" configuration parameter accepting device paths or wildcards
2. WHEN resources is set to specific paths (e.g., ["/dev/nvme0n1"]) THEN the system SHALL monitor only those devices
3. WHEN resources is set to ["*"] THEN the system SHALL automatically discover and monitor all matching Instance Store devices
4. WHEN an invalid device path is specified THEN the system SHALL log an error and skip that device
5. WHEN no resources are configured THEN the system SHALL default to automatic discovery mode

### Requirement 3

**User Story:** As a performance analyst, I want to collect comprehensive NVMe metrics including I/O operations, throughput, latency, and histograms, so that I can analyze storage performance patterns.

#### Acceptance Criteria

1. WHEN log page 0xC0 is retrieved THEN the system SHALL parse and emit the following cumulative metrics:
   - diskio_instance_store_total_read_ops (uint64, count)
   - diskio_instance_store_total_write_ops (uint64, count)
   - diskio_instance_store_total_read_bytes (uint64, bytes)
   - diskio_instance_store_total_write_bytes (uint64, bytes)
   - diskio_instance_store_total_read_time (uint64, nanoseconds)
   - diskio_instance_store_total_write_time (uint64, nanoseconds)
2. WHEN log page 0xC0 is retrieved THEN the system SHALL parse and emit Instance Store specific metrics:
   - diskio_instance_store_volume_performance_exceeded_iops (uint64, count)
   - diskio_instance_store_volume_performance_exceeded_tp (uint64, count)
   - diskio_instance_store_volume_queue_length (uint64, point-in-time count)
3. WHEN histogram data is present in the log page THEN the system SHALL skip histogram parsing and focus only on the basic metrics
4. WHEN cumulative metrics are collected THEN the system SHALL compute deltas between scrapes to provide rate-based values

### Requirement 4

**User Story:** As a CloudWatch user, I want metrics to include relevant dimensions, so that I can filter and aggregate data effectively in CloudWatch dashboards and alarms.

#### Acceptance Criteria

1. WHEN metrics are emitted THEN the system SHALL include the following dimensions:
   - InstanceId (retrieved from EC2 Instance Metadata Service)
   - Device (e.g., "/dev/nvme0n1")
   - SerialNumber (retrieved from NVMe device identification)
2. WHEN metrics are emitted THEN the system SHALL not include histogram-specific dimensions since histogram metrics are not implemented
3. WHEN dimension retrieval fails THEN the system SHALL log an error and use placeholder values where possible

### Requirement 5

**User Story:** As a system administrator, I want the receiver to handle errors gracefully, so that monitoring continues to work even when some devices are inaccessible.

#### Acceptance Criteria

1. WHEN a device cannot be opened THEN the system SHALL log an error and continue with other devices
2. WHEN ioctl calls fail THEN the system SHALL log the failure reason and skip that device for the current scrape
3. WHEN log page magic bytes are invalid (not 0xEC2C0D7E) THEN the system SHALL log an error and skip metric parsing
4. WHEN counter overflow is detected THEN the system SHALL handle the overflow gracefully and log a warning
5. WHEN parsing fails due to insufficient data THEN the system SHALL log an error and skip that metric

### Requirement 6

**User Story:** As a performance-conscious administrator, I want the receiver to have minimal system impact, so that monitoring doesn't affect application performance.

#### Acceptance Criteria

1. WHEN the receiver is running THEN the system SHALL consume less than 1% CPU overhead per 60-second scrape cycle
2. WHEN up to 10 devices are monitored THEN the system SHALL maintain the performance requirements
3. WHEN ioctl operations are performed THEN the system SHALL minimize the time devices are held open
4. WHEN memory is allocated for log page data THEN the system SHALL reuse buffers where possible

### Requirement 7

**User Story:** As a security-conscious administrator, I want the receiver to follow security best practices, so that it doesn't introduce vulnerabilities.

#### Acceptance Criteria

1. WHEN the receiver performs ioctl operations THEN the system SHALL require CAP_SYS_ADMIN capability
2. WHEN the receiver accesses device files THEN the system SHALL validate file paths to prevent directory traversal
3. WHEN the receiver processes log page data THEN the system SHALL validate data bounds to prevent buffer overflows
4. WHEN the receiver fails to obtain required permissions THEN the system SHALL log an appropriate error and fail gracefully

### Requirement 8

**User Story:** As a developer, I want the receiver to reuse existing code patterns, so that maintenance is simplified and consistency is maintained.

#### Acceptance Criteria

1. WHEN implementing NVMe operations THEN the system SHALL reuse code from awsebsnvmereceiver via a shared internal/nvme package
2. WHEN implementing the receiver interface THEN the system SHALL follow the same patterns as other CloudWatch Agent receivers
3. WHEN generating metadata THEN the system SHALL use mdatagen following established conventions
4. WHEN implementing configuration THEN the system SHALL follow the same structure as awsebsnvmereceiver

### Requirement 9

**User Story:** As a quality assurance engineer, I want comprehensive test coverage, so that the receiver is reliable and maintainable.

#### Acceptance Criteria

1. WHEN unit tests are written THEN the system SHALL achieve greater than 90% code coverage
2. WHEN integration tests are created THEN the system SHALL validate metrics appear correctly in CloudWatch
3. WHEN edge cases are tested THEN the system SHALL cover device detection failures, parsing errors, and permission issues
4. WHEN performance tests are run THEN the system SHALL validate CPU and memory usage requirements

### Requirement 10

**User Story:** As a CloudWatch Agent user, I want the receiver to work on supported Linux distributions, so that I can use it in my existing infrastructure.

#### Acceptance Criteria

1. WHEN the receiver runs on Linux THEN the system SHALL support kernels version 4.0 and higher
2. WHEN the receiver runs on EC2 instances THEN the system SHALL work on NVMe-enabled instance types (e.g., i4i, c5d)
3. WHEN the receiver is deployed THEN the system SHALL not require additional IAM permissions beyond existing CloudWatch Agent requirements
4. WHEN the receiver encounters unsupported platforms THEN the system SHALL log appropriate warnings and disable itself gracefully