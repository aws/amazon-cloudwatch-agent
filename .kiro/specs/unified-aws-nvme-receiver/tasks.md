# Implementation Plan

- [x] 1. Set up project structure and core interfaces
  - Create directory structure for the unified awsnvmereceiver package
  - Define interfaces that establish system boundaries for device detection and metrics collection
  - Set up basic factory registration and configuration structures
  - _Requirements: 1.1, 2.1, 5.1_

- [x] 2. Implement unified device type detection in shared utilities
  - Extend internal/nvme package with DetectDeviceType function for unified device identification
  - Implement device type detection logic using model names and magic number validation
  - Add support for both EBS and Instance Store device identification
  - Write unit tests for device type detection with various device scenarios
  - _Requirements: 1.2, 1.4, 5.1_

- [x] 3. Create unified metrics structures and parsing logic
  - Define EBSMetrics struct with exact structure including histogram fields
  - Define InstanceStoreMetrics struct similar to EBS but skipping EBS-specific fields
  - Implement parsing functions using binary.LittleEndian for both device types
  - Add magic number validation for Instance Store devices (0xEC2C0D7E)
  - Write unit tests for metrics structure parsing and validation
  - _Requirements: 1.3, 5.6, 6.4_

- [x] 4. Implement unified receiver factory and configuration
  - Create factory.go with NewFactory function for "awsnvmereceiver" type
  - Implement Config struct supporting existing diskio configuration parameters
  - Add validation logic for devices configuration (wildcard and specific paths)
  - Ensure backward compatibility with existing awsebsnvmereceiver configurations
  - Write unit tests for factory creation and configuration validation
  - _Requirements: 2.1, 2.2, 5.4, 10.2_

- [x] 5. Create comprehensive metadata.yaml for unified metrics
  - Define metadata.yaml with all EBS and Instance Store metrics using appropriate prefixes
  - Include resource attributes for instance_id, device_type, device, and serial_number
  - Generate MetricsBuilder code using mdatagen tool
  - Ensure metric definitions match the provided JSON configuration measurement list
  - _Requirements: 3.1, 3.2, 4.1, 5.3_

- [ ] 6. Implement core scraper with device type routing
  - Create scraper.go with unified scraping logic and device type detection
  - Implement getDevicesByController for device discovery and grouping
  - Add device type routing logic to call appropriate parsing functions
  - Implement safe metric recording with overflow protection and prefix application
  - Write unit tests for scraper functionality with mixed device scenarios
  - _Requirements: 1.1, 1.5, 3.4, 6.1, 6.3_

- [ ] 7. Add comprehensive error handling and logging
  - Implement graceful error handling for device access failures and ioctl operations
  - Add detailed logging for device type detection failures and parsing errors
  - Implement counter overflow detection and handling
  - Add platform-specific graceful degradation for unsupported systems
  - Write unit tests for error scenarios and recovery mechanisms
  - _Requirements: 6.1, 6.2, 6.3, 6.4, 10.5_

- [ ] 8. Implement security measures and input validation
  - Add device path validation to prevent directory traversal attacks
  - Implement log page data bounds validation for both EBS and Instance Store formats
  - Add input sanitization for all configuration parameters
  - Ensure proper capability requirements (CAP_SYS_ADMIN) are documented
  - Write security-focused unit tests for input validation and bounds checking
  - _Requirements: 8.1, 8.2, 8.3, 8.4_

- [ ] 9. Create comprehensive unit test suite
  - Write unit tests achieving >90% code coverage for all components
  - Create mock DeviceInfoProvider for testing both device types
  - Add test cases for mixed device environments and edge cases
  - Include performance tests validating resource usage requirements
  - Test device discovery with wildcard (*) and specific device paths
  - _Requirements: 9.1, 9.2, 9.3, 9.4_

- [ ] 10. Integrate receiver into existing pipeline and validate compatibility
  - Update service registration to include awsnvmereceiver in default components
  - Ensure integration with existing processors (cumulativetodelta, ec2tagger, awsentity)
  - Validate compatibility with awscloudwatch exporter
  - Test pipeline integration with mixed device configurations
  - Write integration tests for end-to-end metric flow
  - _Requirements: 10.1, 10.3, 10.4_

- [ ] 11. Performance optimization and validation
  - Implement device type caching to avoid repeated expensive operations
  - Add buffer reuse for log page operations across device types
  - Optimize device grouping and ioctl batching where possible
  - Validate CPU usage <1% and memory usage <10MB requirements
  - Ensure scrape latency <50ms for 10 mixed devices
  - _Requirements: 7.1, 7.2, 7.3, 7.4_

- [-] 12. Create migration documentation and backward compatibility validation
  - Document migration steps from existing awsebsnvmereceiver and awsinstancestorenvmereceiver
  - Validate that existing EBS configurations work without modification
  - Test metric name consistency and dimension compatibility
  - Ensure no breaking changes for existing users
  - Create deployment and rollback procedures
  - _Requirements: 5.4, 10.2, 10.5_