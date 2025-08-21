# Design Document

## Overview

This feature implements dynamic User-Agent feature flag appending for the CloudWatch Agent's PutMetricData requests. The system will analyze metrics at runtime to detect EBS and instance store disk I/O patterns, then append corresponding feature flags ('nvme_ebs', 'nvme_is') to the User-Agent header. This provides AWS with visibility into which storage types are actively being monitored by each agent instance.

## Architecture

The feature flag detection and User-Agent modification will be implemented within the existing CloudWatch output plugin architecture. The solution leverages the existing request handling pipeline and adds metric analysis capabilities without disrupting the current flow.

### High-Level Flow

1. **Metric Processing**: During the `WriteToCloudWatch` method execution, analyze the metrics in the PutMetricData request
2. **Feature Detection**: Check for EBS and instance store metric prefixes in the metric names
3. **Configuration Validation**: Verify that the corresponding diskio features are enabled in the agent configuration
4. **User-Agent Construction**: Build the feature flag string and append it to the User-Agent header
5. **Request Execution**: Send the request with the modified User-Agent header

## Components and Interfaces

### 1. Feature Flag Detector

**Location**: `plugins/outputs/cloudwatch/cloudwatch.go`

**Interface**:
```go
type FeatureFlagDetector interface {
    DetectFeatureFlags(metricData []*cloudwatch.MetricDatum, entityMetricData []*cloudwatch.EntityMetricData) []string
}
```

**Implementation**:
- Analyzes metric names in both regular MetricData and EntityMetricData
- Returns a slice of detected feature flags
- Optimized for performance with early exit conditions

### 2. Configuration Checker

**Interface**:
```go
type ConfigChecker interface {
    IsDiskIOFeatureEnabled(featureType string) bool
}
```

**Implementation**:
- Checks agent configuration to determine if EBS or instance store monitoring is enabled
- Returns boolean indicating feature enablement status
- Integrates with existing configuration management

### 3. User-Agent Builder

**Interface**:
```go
type UserAgentBuilder interface {
    BuildFeatureFlagString(flags []string) string
}
```

**Implementation**:
- Formats feature flags into the required string format: `feature:(flag1,flag2)`
- Handles empty flag lists gracefully
- Ensures consistent formatting

### 4. Dynamic Header Handler

**Location**: `handlers/customheader.go` (extension)

**Implementation**:
- Extends existing custom header functionality
- Provides dynamic User-Agent modification capability
- Integrates with AWS SDK request pipeline

## Data Models

### Feature Flag Constants

```go
const (
    FeatureFlagNvmeEBS = "nvme_ebs"
    FeatureFlagNvmeIS  = "nvme_is"
    
    MetricPrefixEBS           = "diskio_ebs_"
    MetricPrefixInstanceStore = "diskio_instance_store_"
    
    FeatureFlagPrefix = "feature:"
)
```

### Feature Detection Result

```go
type FeatureDetectionResult struct {
    HasEBSMetrics           bool
    HasInstanceStoreMetrics bool
    DetectedFlags           []string
}
```

## Error Handling

### Graceful Degradation
- If feature flag detection fails, the request proceeds without feature flags
- Errors are logged but do not block metric publishing
- Performance issues trigger fallback to no feature flag detection

### Error Scenarios
1. **Metric Analysis Failure**: Log error, continue without feature flags
2. **Configuration Access Failure**: Assume features are disabled, continue
3. **User-Agent Construction Failure**: Log error, use original User-Agent
4. **Header Modification Failure**: Log error, continue with original headers

### Logging Strategy
- Debug level: Feature flag detection results
- Info level: Configuration-related decisions
- Warning level: Performance degradation or fallback scenarios
- Error level: Unexpected failures that don't block requests

## Testing Strategy

### Unit Tests

1. **Feature Flag Detection Tests**
   - Test EBS metric prefix detection
   - Test instance store metric prefix detection
   - Test mixed metric scenarios
   - Test empty metric scenarios
   - Test performance with large metric batches

2. **Configuration Integration Tests**
   - Test with EBS features enabled/disabled
   - Test with instance store features enabled/disabled
   - Test with both features enabled/disabled
   - Test configuration access failures

3. **User-Agent Construction Tests**
   - Test single flag formatting
   - Test multiple flag formatting
   - Test empty flag list handling
   - Test special character handling

4. **Header Handler Tests**
   - Test User-Agent modification
   - Test header preservation
   - Test request pipeline integration

### Integration Tests

1. **End-to-End Request Tests**
   - Test PutMetricData requests with EBS metrics
   - Test PutMetricData requests with instance store metrics
   - Test PutMetricData requests with mixed metrics
   - Test PutMetricData requests with no diskio metrics

2. **Performance Tests**
   - Measure feature flag detection overhead
   - Test with various batch sizes
   - Verify no significant latency impact

3. **Configuration Integration Tests**
   - Test with real agent configuration files
   - Test configuration changes at runtime
   - Test invalid configuration scenarios

### Mock Strategy

- Mock CloudWatch API calls to verify User-Agent headers
- Mock configuration access for testing various scenarios
- Mock metric data for consistent test scenarios
- Use table-driven tests for comprehensive coverage

## Implementation Details

### Metric Analysis Algorithm

```go
func (c *CloudWatch) detectFeatureFlags(params *cloudwatch.PutMetricDataInput) []string {
    var flags []string
    hasEBS := false
    hasInstanceStore := false
    
    // Check regular MetricData
    for _, metric := range params.MetricData {
        if metric.MetricName != nil {
            name := *metric.MetricName
            if strings.HasPrefix(name, MetricPrefixEBS) {
                hasEBS = true
            } else if strings.HasPrefix(name, MetricPrefixInstanceStore) {
                hasInstanceStore = true
            }
            
            // Early exit if both found
            if hasEBS && hasInstanceStore {
                break
            }
        }
    }
    
    // Check EntityMetricData if needed
    if (!hasEBS || !hasInstanceStore) && params.EntityMetricData != nil {
        // Similar analysis for entity metrics
    }
    
    // Build flags based on configuration and detection
    if hasEBS && c.isEBSFeatureEnabled() {
        flags = append(flags, FeatureFlagNvmeEBS)
    }
    if hasInstanceStore && c.isInstanceStoreFeatureEnabled() {
        flags = append(flags, FeatureFlagNvmeIS)
    }
    
    return flags
}
```

### User-Agent Header Modification

The implementation will use the existing custom header handler pattern:

```go
func (c *CloudWatch) createUserAgentHandler() request.NamedHandler {
    return request.NamedHandler{
        Name: "DynamicUserAgentHandler",
        Fn: func(req *request.Request) {
            if req.Operation.Name == opPutMetricData {
                if params, ok := req.Params.(*cloudwatch.PutMetricDataInput); ok {
                    flags := c.detectFeatureFlags(params)
                    if len(flags) > 0 {
                        featureString := c.buildFeatureFlagString(flags)
                        currentUA := req.HTTPRequest.Header.Get("User-Agent")
                        newUA := currentUA + " " + featureString
                        req.HTTPRequest.Header.Set("User-Agent", newUA)
                    }
                }
            }
        },
    }
}
```

### Configuration Integration

The feature will integrate with the existing configuration system by adding methods to check diskio feature enablement:

```go
func (c *CloudWatch) isEBSFeatureEnabled() bool {
    // Check agent configuration for EBS diskio enablement
    // This will require integration with the agent's configuration system
    // Implementation details depend on how diskio features are configured
    return true // Placeholder - actual implementation will check config
}

func (c *CloudWatch) isInstanceStoreFeatureEnabled() bool {
    // Check agent configuration for instance store diskio enablement
    return true // Placeholder - actual implementation will check config
}
```

### Performance Optimizations

1. **Early Exit**: Stop analysis once both feature types are detected
2. **Prefix Matching**: Use efficient string prefix matching
3. **Caching**: Cache configuration checks to avoid repeated lookups
4. **Conditional Analysis**: Skip analysis if no diskio features are enabled
5. **String Builder**: Use efficient string building for User-Agent construction

### Backward Compatibility

- No changes to existing API interfaces
- No changes to configuration file formats
- Feature flags are additive to existing User-Agent strings
- Graceful handling when features are disabled or not configured