# Implementing the Raw Normalized Load Average Metric

## Overview
This document outlines the implementation of a raw normalized load average metric for the Amazon CloudWatch Agent. The metric will emit the 1-minute load average divided by the number of CPU cores, making it easily interpretable for customers.

## Requirements

### Functional Requirements
- Uses `system.cpu.load_average.1m` OTel plugin for Linux and Mac
- Works alongside existing Telegraf CPU metrics under the same CPU dimensions and time intervals
- Divides load average by CPU cores using the CPUAverage flag
- Maps `system.cpu.load_average.1m` to the metric name `cpu_load_average`
- Only emits 1-minute interval (not 5m or 15m)
- Configurable via JSON configuration

### Configuration Format
```json
{
  "metrics": {
    "metrics_collected": {
      "cpu": {
        "measurement": ["cpu_load_average"],
        "metrics_collection_interval": 60
      }
    }
  }
}
```

### Architecture Requirements
- Follow similar architecture pattern as EBS metrics
- Create minimal new files:
  - Receiver for hostmetrics translator
  - Test file for the receiver
  - YAML processor configuration for dimensions and renaming
- Remaining changes should be edits to existing files
- Design should be extensible for future load average metrics

## Implementation Plan

### 1. New Files to Create

#### Receiver Implementation
- **File**: `translator/translate/otel/processor/hostmetricsreceiver/cpu_load_average.go`
- **Purpose**: Translate OTel `system.cpu.load_average.1m` to CloudWatch `cpu_load_average`
- **Key Features**:
  - Handle normalization by CPU core count
  - Apply CPUAverage flag logic
  - Map metric names appropriately

#### Test File
- **File**: `translator/translate/otel/processor/hostmetricsreceiver/cpu_load_average_test.go`
- **Purpose**: Unit tests for the load average receiver
- **Coverage**:
  - Metric name mapping
  - CPU normalization logic
  - Configuration parsing
  - Edge cases (single core, high load, etc.)

#### Processor Configuration
- **File**: `translator/translate/otel/processor/hostmetricsreceiver/cpu_load_average.yaml`
- **Purpose**: Define dimensions and metric renaming rules
- **Content**:
  - Dimension mappings
  - Metric name transformations
  - Default configurations

### 2. Files to Modify

#### Configuration Parsing
- **Files to edit**:
  - `cfg/envconfig/envconfig.go` - Add cpu_load_average to measurement types
  - `cfg/commonconfig/commonconfig.go` - Update CPU measurement validation
  - Configuration validation logic

#### Metric Registration
- **Files to edit**:
  - Metric registry files to include the new load average metric
  - CPU metric collection initialization
  - Hostmetrics receiver configuration

#### Documentation Strings
- **Files to edit**:
  - String constants files for metric names
  - Error message definitions
  - Configuration help text

### 3. Integration Points

#### With Existing CPU Metrics
- Ensure load average metrics appear under the same CPU dimensions
- Maintain consistent time intervals with other CPU metrics
- Preserve existing Telegraf CPU metric functionality

#### With OTel Hostmetrics Receiver
- Leverage existing `system.cpu.load_average.1m` collection
- Apply CPU core normalization in the translation layer
- Handle platform-specific differences (Linux vs Mac)

## Technical Details

### Metric Normalization Logic
```go
normalizedLoadAverage = rawLoadAverage / cpuCoreCount
```

### Metric Naming Convention
- OTel Source: `system.cpu.load_average.1m`
- CloudWatch Target: `cpu_load_average`
- Dimension: Same as existing CPU metrics

### Platform Support
- **Linux**: Full support via `/proc/loadavg`
- **macOS**: Full support via system calls
- **Windows**: Not applicable (load average concept doesn't exist)

## Future Extensibility

The implementation should be designed to easily accommodate:
- 5-minute load average (`system.cpu.load_average.5m` → `cpu_load_average_5m`)
- 15-minute load average (`system.cpu.load_average.15m` → `cpu_load_average_15m`)
- Additional load-related metrics
- Different normalization strategies

## Testing Strategy

### Unit Tests
- Metric name mapping validation
- CPU core count normalization
- Configuration parsing
- Error handling

### Integration Tests
- End-to-end metric collection
- CloudWatch emission verification
- Compatibility with existing CPU metrics

### Platform Tests
- Linux-specific load average collection
- macOS-specific load average collection
- Cross-platform consistency

## Documentation Updates

### Customer-Facing Documentation
- Explain load average interpretation
- Provide examples for different CPU core counts
- Configuration examples and best practices

### Internal Documentation
- Code comments explaining normalization logic
- Architecture decision records
- Maintenance guidelines

## Success Criteria

1. ✅ Load average metric successfully collected on Linux and macOS
2. ✅ Metric properly normalized by CPU core count
3. ✅ Configuration works as specified in JSON format
4. ✅ Metric appears under CPU dimensions alongside existing metrics
5. ✅ No impact on existing CPU metric collection
6. ✅ Comprehensive test coverage
7. ✅ Clear documentation for customers and maintainers