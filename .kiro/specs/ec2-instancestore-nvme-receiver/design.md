# Design Document

## Overview

The EC2 Instance Store NVMe receiver (`awsinstancestorenvmereceiver`) extends the Amazon CloudWatch Agent to collect performance metrics from EC2 Instance Store NVMe devices. This receiver follows the same architectural patterns as the existing `awsebsnvmereceiver` but targets Instance Store volumes specifically, using the magic number 0xEC2C0D7E to identify and validate Instance Store devices.

The receiver integrates into the existing `diskio` metrics collection pipeline, allowing users to configure both EBS and Instance Store metrics through a unified interface. It leverages shared NVMe utilities to minimize code duplication while providing Instance Store-specific functionality.

## Architecture

### High-Level Architecture

```mermaid
graph TB
    A[CloudWatch Agent] --> B[Metrics Pipeline]
    B --> C[awsinstancestorenvmereceiver]
    C --> D[NVMe Device Scanner]
    C --> E[Instance Store Scraper]
    C --> F[Metrics Builder]
    
    D --> G[/dev/nvme* devices]
    E --> H[Log Page 0xC0]
    F --> I[OpenTelemetry Metrics]
    I --> J[CloudWatch]
    
    K[Shared NVMe Utils] --> C
    K --> L[awsebsnvmereceiver]
```

### Component Integration

The receiver integrates into the existing CloudWatch Agent architecture as follows:

1. **Configuration**: Extends the existing `diskio` configuration section
2. **Pipeline**: Runs alongside `awsebsnvmereceiver` in the `metrics/hostDeltaMetrics` pipeline
3. **Shared Utilities**: Reuses NVMe device discovery and ioctl operations from shared `internal/nvme` package
4. **Metrics Output**: Emits OpenTelemetry metrics with consistent naming and dimensions

## Components and Interfaces

### Core Components

#### 1. Factory (`factory.go`)
- **Purpose**: Creates and configures the receiver instance
- **Interface**: Implements `receiver.Factory` from OpenTelemetry Collector
- **Key Functions**:
  - `NewFactory()`: Returns configured factory
  - `createDefaultConfig()`: Provides default configuration
  - `createMetricsReceiver()`: Instantiates the metrics receiver

#### 2. Configuration (`config.go`)
- **Purpose**: Defines receiver configuration structure
- **Key Fields**:
  - `Devices []string`: List of device paths or "*" for auto-discovery
  - `ControllerConfig`: Scraping interval and timeout settings
  - `MetricsBuilderConfig`: Metric enablement configuration

#### 3. Scraper (`scraper.go`)
- **Purpose**: Core logic for device discovery, metric collection, and emission
- **Key Functions**:
  - `scrape()`: Main scraping logic executed on schedule
  - `getInstanceStoreDevicesByController()`: Device discovery and validation
  - `recordMetric()`: Safe metric recording with overflow protection

#### 4. Metadata (`metadata.yaml`)
- **Purpose**: Defines available metrics, resource attributes, and configuration schema
- **Generated Code**: Produces `MetricsBuilder` and related types via `mdatagen`

### Shared Components

#### 1. NVMe Utilities (`internal/nvme`)
- **Interface**: `DeviceInfoProvider` for device operations
- **Key Functions**:
  - `GetAllDevices()`: Discover all NVMe devices
  - `GetDeviceModel()`: Extract device model name
  - `GetDeviceSerial()`: Extract device serial number
  - `DevicePath()`: Convert device name to full path

#### 2. Instance Store Metrics (`internal/nvme/instance_store_metrics.go`)
- **Purpose**: Define Instance Store-specific metric structure and parsing
- **Key Types**:
  - `InstanceStoreMetrics`: Parsed log page structure
  - `GetInstanceStoreMetrics()`: Log page retrieval and parsing function

### Interface Definitions

```go
// DeviceInfoProvider interface (shared)
type DeviceInfoProvider interface {
    GetAllDevices() ([]DeviceFileAttributes, error)
    GetDeviceSerial(*DeviceFileAttributes) (string, error)
    GetDeviceModel(*DeviceFileAttributes) (string, error)
    IsInstanceStoreDevice(*DeviceFileAttributes) (bool, error)
    DevicePath(string) (string, error)
}

// Instance Store specific metrics structure
type InstanceStoreMetrics struct {
    Magic                    uint32  // 0xEC2C0D7E
    Reserved                 uint32
    ReadOps                  uint64
    WriteOps                 uint64
    ReadBytes                uint64
    WriteBytes               uint64
    TotalReadTime            uint64
    TotalWriteTime           uint64
    EBSIOPSExceeded          uint64  // Skip - not applicable
    EBSThroughputExceeded    uint64  // Skip - not applicable
    EC2IOPSExceeded          uint64
    EC2ThroughputExceeded    uint64
    QueueLength              uint64
    // Histogram data (skipped in initial implementation)
}
```

## Data Models

### Configuration Model

```yaml
# Agent configuration structure
metrics:
  namespace: "EC2InstanceStoreMetrics"
  metrics_collected:
    diskio:
      resources: ["*"]  # or specific devices like ["/dev/nvme0n1"]
      measurement:
        - "diskio_instance_store_total_read_ops"
        - "diskio_instance_store_total_write_ops"
        # ... other metrics
      metrics_collection_interval: 60
```

### Device Discovery Model

```go
type instanceStoreDevices struct {
    serialNumber string
    deviceNames  []string
}

// Grouped by controller ID to avoid duplicate metrics
type devicesByController map[int]*instanceStoreDevices
```

### Metrics Model

All metrics follow OpenTelemetry conventions:

- **Type**: Sum (cumulative, monotonic) for counters, Gauge for point-in-time values
- **Value Type**: int64 (with overflow protection)
- **Dimensions**: InstanceId, Device, SerialNumber
- **Units**: Standard units (bytes, nanoseconds, count)

### Log Page Structure

The Instance Store log page (ID 0xC0) follows this binary layout:

```
Offset | Size | Field                    | Type   | Usage
-------|------|--------------------------|--------|------------------
0      | 4    | Magic                    | uint32 | 0xEC2C0D7E validation
4      | 4    | Reserved                 | uint32 | Skip
8      | 8    | Read Ops                 | uint64 | Cumulative counter
16     | 8    | Write Ops                | uint64 | Cumulative counter
24     | 8    | Read Bytes               | uint64 | Cumulative counter
32     | 8    | Write Bytes              | uint64 | Cumulative counter
40     | 8    | Total Read Time          | uint64 | Cumulative (ns)
48     | 8    | Total Write Time         | uint64 | Cumulative (ns)
56     | 8    | EBS IOPS Exceeded        | uint64 | Skip (not applicable)
64     | 8    | EBS Throughput Exceeded  | uint64 | Skip (not applicable)
72     | 8    | EC2 IOPS Exceeded        | uint64 | Cumulative counter
80     | 8    | EC2 Throughput Exceeded  | uint64 | Cumulative counter
88     | 8    | Queue Length             | uint64 | Point-in-time gauge
96+    | ...  | Histogram Data           | ...    | Skip (not implemented)
```

## Error Handling

### Error Categories and Responses

1. **Device Access Errors**
   - **Cause**: Permission denied, device not found
   - **Response**: Log error, skip device, continue with others
   - **Recovery**: Retry on next scrape cycle

2. **Parsing Errors**
   - **Cause**: Invalid magic number, insufficient data
   - **Response**: Log error, skip metric parsing
   - **Recovery**: Retry on next scrape cycle

3. **Overflow Errors**
   - **Cause**: uint64 values exceeding int64 maximum
   - **Response**: Log warning, skip metric
   - **Recovery**: Continue with other metrics

4. **Configuration Errors**
   - **Cause**: Invalid device paths, malformed config
   - **Response**: Log error, use defaults where possible
   - **Recovery**: Validate and sanitize configuration

### Error Handling Patterns

```go
// Safe metric recording with overflow protection
func (s *scraper) recordMetric(recordFn recordDataMetricFunc, ts pcommon.Timestamp, val uint64) {
    converted, err := safeUint64ToInt64(val)
    if err != nil {
        s.logger.Debug("skipping metric due to potential integer overflow")
        return
    }
    recordFn(ts, converted)
}

// Device validation with graceful failure
func (s *scraper) validateInstanceStoreDevice(device *DeviceFileAttributes) bool {
    isInstanceStore, err := s.nvme.IsInstanceStoreDevice(device)
    if err != nil {
        s.logger.Debug("unable to validate device", zap.String("device", device.DeviceName()), zap.Error(err))
        return false
    }
    return isInstanceStore
}
```

## Testing Strategy

### Unit Testing Approach

1. **Mock-Based Testing**
   - Mock `DeviceInfoProvider` interface for device operations
   - Mock ioctl calls with sample log page data
   - Test all error conditions and edge cases

2. **Test Coverage Areas**
   - Device discovery and filtering
   - Log page parsing and validation
   - Metric recording and overflow handling
   - Configuration validation
   - Error handling paths

3. **Test Data**
   - Sample log page binary data with valid magic number
   - Invalid log page data for error testing
   - Device identification data for filtering tests

### Integration Testing Approach

1. **EC2 Environment Testing**
   - Deploy on Instance Store-enabled instances (i4i, c5d)
   - Validate metrics appear in CloudWatch
   - Compare with `nvme-cli` output for accuracy

2. **Performance Testing**
   - Measure CPU and memory usage during scraping
   - Test with multiple devices (up to 10)
   - Validate <1% CPU overhead requirement

3. **End-to-End Testing**
   - Full agent configuration and deployment
   - CloudWatch metrics validation
   - Dashboard and alarm functionality

### Test Structure

```
receiver/awsinstancestorenvmereceiver/
├── config_test.go              # Configuration validation tests
├── factory_test.go             # Factory creation tests
├── scraper_test.go             # Core scraping logic tests
├── testdata/
│   ├── sample_log_page.bin     # Valid Instance Store log page
│   ├── invalid_log_page.bin    # Invalid magic number test data
│   └── device_info.json        # Sample device information
└── internal/
    └── nvme/
        ├── instance_store_metrics_test.go  # Parsing tests
        └── testdata/
            └── log_pages/              # Additional test data
```

## Implementation Phases

### Phase 1: Core Infrastructure (Days 1-2)
- Create receiver package structure
- Implement basic factory and configuration
- Set up shared NVMe utilities extension
- Create metadata.yaml and generate code

### Phase 2: Device Discovery (Days 2-3)
- Implement Instance Store device identification
- Add magic number validation
- Create device grouping by controller ID
- Add comprehensive error handling

### Phase 3: Metrics Collection (Days 3-4)
- Implement log page parsing
- Add metric recording with overflow protection
- Create OpenTelemetry metric emission
- Add dimension handling (InstanceId, Device, SerialNumber)

### Phase 4: Testing and Integration (Days 4-8)
- Write comprehensive unit tests
- Create integration tests for EC2 environment
- Performance testing and optimization
- Documentation and PR preparation

## Security Considerations

### Required Permissions
- **CAP_SYS_ADMIN**: Required for NVMe ioctl operations
- **Device Access**: Read access to `/dev/nvme*` devices
- **No Additional IAM**: Uses existing CloudWatch Agent permissions

### Security Measures
- **Path Validation**: Prevent directory traversal in device paths
- **Buffer Bounds**: Validate log page data size before parsing
- **Input Sanitization**: Validate all configuration inputs
- **Privilege Minimization**: Only request required capabilities

### Risk Mitigation
- **Graceful Degradation**: Continue operation if some devices are inaccessible
- **Resource Limits**: Limit number of devices processed simultaneously
- **Error Isolation**: Prevent errors in one device from affecting others

## Performance Considerations

### Optimization Strategies

1. **Device Grouping**: Group devices by controller ID to avoid duplicate work
2. **Buffer Reuse**: Reuse log page buffers across scrape cycles
3. **Selective Parsing**: Only parse enabled metrics from log page
4. **Efficient Logging**: Use structured logging with appropriate levels

### Resource Management

- **Memory**: Limit log page buffer allocation (4KB per device)
- **CPU**: Minimize ioctl call frequency and duration
- **I/O**: Batch device operations where possible
- **Concurrency**: Process devices sequentially to avoid resource contention

### Performance Targets

- **CPU Usage**: <1% per 60-second scrape cycle
- **Memory Usage**: <10MB additional memory footprint
- **Latency**: <100ms total scrape time for 10 devices
- **Scalability**: Support up to 10 Instance Store devices per instance

This design provides a robust, maintainable, and performant solution for collecting Instance Store NVMe metrics while following established patterns and maintaining consistency with the existing CloudWatch Agent architecture.