# Implementation Plan

- [x] 1. Set up receiver package structure and shared utilities
  - Create the awsinstancestorenvmereceiver package directory structure
  - Extend shared internal/nvme package with Instance Store specific functions
  - Create basic package files with copyright headers and imports
  - _Requirements: 8.1, 8.2_

- [x] 2. Create Instance Store metrics data structures
  - Define InstanceStoreMetrics struct matching log page layout (bytes 0-95)
  - Implement log page parsing function with magic number validation (0xEC2C0D7E)
  - Add safe uint64 to int64 conversion with overflow detection
  - Create unit tests for parsing logic with sample log page data
  - _Requirements: 3.1, 3.4, 5.4_

- [x] 3. Implement device identification and validation
  - Add IsInstanceStoreDevice function to shared nvme utilities
  - Implement model name checking for "Amazon EC2 NVMe Instance Storage"
  - Add magic number validation from log page 0xC0
  - Create unit tests for device identification with mock data
  - _Requirements: 1.1, 1.2, 1.3, 5.3_

- [x] 4. Create receiver configuration structure
  - Implement Config struct with Devices field and embedded configs
  - Add configuration validation for device paths and wildcard support
  - Implement createDefaultConfig function with empty devices list
  - Create unit tests for configuration validation and defaults
  - _Requirements: 2.1, 2.2, 2.5, 5.1_

- [x] 5. Implement receiver factory
  - Create NewFactory function returning configured receiver.Factory
  - Implement createMetricsReceiver with scraper initialization
  - Add proper error handling for factory creation failures
  - Create unit tests for factory creation and error conditions
  - _Requirements: 8.2, 5.1_

- [x] 6. Create metadata definition and generate code
  - Write metadata.yaml with all Instance Store metrics definitions
  - Define resource attributes (InstanceId, Device, SerialNumber)
  - Configure metric types (sum/gauge), units, and default enablement
  - Run mdatagen to generate MetricsBuilder and related code
  - _Requirements: 3.1, 3.2, 4.1, 8.3_

- [x] 7. Implement core scraper logic
  - Create nvmeScraper struct with logger, metrics builder, and device provider
  - Implement start and shutdown lifecycle methods
  - Add device discovery with controller ID grouping logic
  - Create unit tests for scraper initialization and lifecycle
  - _Requirements: 1.4, 1.5, 6.3, 8.2_

- [x] 8. Implement device discovery and filtering
  - Create getInstanceStoreDevicesByController function
  - Add device filtering based on configuration (specific paths vs wildcard)
  - Implement controller ID grouping to avoid duplicate metrics
  - Add comprehensive error handling for device access failures
  - Create unit tests with mock device provider and various configurations
  - _Requirements: 1.1, 1.2, 2.1, 2.2, 2.3, 2.4, 5.1, 5.2_

- [x] 9. Implement log page retrieval and parsing
  - Add GetInstanceStoreMetrics function using NVME_IOCTL_ADMIN_CMD
  - Implement log page 0xC0 retrieval with 4KB buffer allocation
  - Add binary parsing with little-endian byte order for all fields
  - Skip EBS-specific fields (bytes 56-71) and histogram data (96+)
  - Create unit tests with sample binary log page data
  - _Requirements: 3.1, 3.3, 5.3, 5.5, 6.4_

- [x] 10. Implement metric recording and emission
  - Add recordMetric function with safe uint64 to int64 conversion
  - Implement all 9 Instance Store metrics recording in scrape function
  - Add dimension setting (InstanceId from IMDS, Device path, SerialNumber)
  - Create resource builder and emit metrics via MetricsBuilder
  - Create unit tests for metric recording and dimension handling
  - _Requirements: 3.1, 3.2, 4.1, 4.2, 5.4_

- [x] 11. Add comprehensive error handling
  - Implement graceful handling for device access failures
  - Add logging for ioctl failures with appropriate error levels
  - Handle invalid magic number validation with error logging
  - Add counter overflow detection and warning logs
  - Create unit tests covering all error scenarios
  - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5_

- [ ] 12. Implement security and permission validation
  - Add device path validation to prevent directory traversal
  - Implement buffer bounds checking for log page parsing
  - Add CAP_SYS_ADMIN requirement documentation in error messages
  - Create unit tests for security validation edge cases
  - _Requirements: 7.1, 7.2, 7.3, 7.4_

- [ ] 13. Add performance optimizations
  - Implement buffer reuse for log page data across scrape cycles
  - Add device handle management to minimize open/close operations
  - Optimize device grouping to reduce redundant ioctl calls
  - Create performance tests measuring CPU and memory usage
  - _Requirements: 6.1, 6.2, 6.3, 6.4_

- [ ] 14. Create comprehensive unit test suite
  - Write tests for all public functions with >90% coverage
  - Create mock implementations for DeviceInfoProvider interface
  - Add test data files with valid and invalid log page samples
  - Implement table-driven tests for various device configurations
  - _Requirements: 9.1, 9.3_

- [ ] 15. Add integration with existing receiver registry
  - Update service/defaultcomponents/components.go to include new receiver
  - Add receiver to build configuration and import statements
  - Ensure receiver is properly registered in component factory
  - Create integration test validating receiver registration
  - _Requirements: 8.2, 8.3_

- [ ] 16. Create integration tests for EC2 environment
  - Write integration tests that can run on Instance Store-enabled EC2 instances
  - Add CloudWatch metrics validation comparing with nvme-cli output
  - Create test configuration files for various device scenarios
  - Implement performance validation tests for CPU/memory requirements
  - _Requirements: 9.2, 6.1, 6.2, 10.1, 10.2_

- [ ] 17. Add documentation and examples
  - Update README with receiver configuration examples
  - Add inline code documentation for all public functions
  - Create configuration examples for different use cases
  - Document required permissions and setup instructions
  - _Requirements: 7.1, 10.3, 10.4_

- [ ] 18. Final integration and testing
  - Run full test suite and ensure >90% coverage
  - Test receiver with complete CloudWatch Agent configuration
  - Validate metrics appear correctly in CloudWatch console
  - Perform final performance validation on EC2 instances
  - _Requirements: 9.1, 9.2, 6.1, 6.2_