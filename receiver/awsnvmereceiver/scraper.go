// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsnvmereceiver

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
	"github.com/aws/amazon-cloudwatch-agent/internal/nvme"
	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsnvmereceiver/internal/metadata"
)

// deviceTypeCache caches device type detection results to avoid repeated expensive operations
type deviceTypeCache struct {
	cache map[string]deviceTypeCacheEntry
	mutex sync.RWMutex
}

type deviceTypeCacheEntry struct {
	deviceType string
	timestamp  time.Time
	ttl        time.Duration
}

// bufferPool manages reusable buffers for log page operations
type bufferPool struct {
	pool sync.Pool
}

// nvmeScraper implements unified scraping logic for both EBS and Instance Store NVMe devices
type nvmeScraper struct {
	logger           *zap.Logger
	mb               *metadata.MetricsBuilder
	nvmeUtil         nvme.DeviceInfoProvider
	metadataProvider ec2metadataprovider.MetadataProvider
	deviceSet        collections.Set[string]

	// Performance optimization components
	deviceTypeCache *deviceTypeCache
	bufferPool      *bufferPool
	lastScrapeTime  time.Time
	scrapeCount     int64
}

// nvmeDevices represents a group of devices with the same controller ID and device type
type nvmeDevices struct {
	deviceType   string   // "ebs" or "instance_store"
	serialNumber string   // Device serial number or volume ID
	deviceNames  []string // List of device names with same controller
}

// devicesByController maps controller ID to device information
type devicesByController map[int]*nvmeDevices

// recordDataMetricFunc defines the function signature for recording metrics
type recordDataMetricFunc func(pcommon.Timestamp, int64)

// newDeviceTypeCache creates a new device type cache
func newDeviceTypeCache() *deviceTypeCache {
	return &deviceTypeCache{
		cache: make(map[string]deviceTypeCacheEntry),
	}
}

// get retrieves a cached device type if it's still valid
func (c *deviceTypeCache) get(deviceKey string) (string, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	entry, exists := c.cache[deviceKey]
	if !exists {
		return "", false
	}

	// Check if cache entry has expired
	if time.Since(entry.timestamp) > entry.ttl {
		return "", false
	}

	return entry.deviceType, true
}

// set stores a device type in the cache
func (c *deviceTypeCache) set(deviceKey, deviceType string, ttl time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.cache[deviceKey] = deviceTypeCacheEntry{
		deviceType: deviceType,
		timestamp:  time.Now(),
		ttl:        ttl,
	}
}

// clear removes expired entries from the cache
func (c *deviceTypeCache) clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now()
	for key, entry := range c.cache {
		if now.Sub(entry.timestamp) > entry.ttl {
			delete(c.cache, key)
		}
	}
}

// newBufferPool creates a new buffer pool for log page operations
func newBufferPool() *bufferPool {
	return &bufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				// Allocate 4KB buffer for NVMe log pages
				return make([]byte, 4096)
			},
		},
	}
}

// getBuffer retrieves a buffer from the pool
func (p *bufferPool) getBuffer() []byte {
	return p.pool.Get().([]byte)
}

// putBuffer returns a buffer to the pool
func (p *bufferPool) putBuffer(buf []byte) {
	// Reset buffer to ensure no data leakage
	for i := range buf {
		buf[i] = 0
	}
	p.pool.Put(buf)
}

// newScraper creates a new unified NVMe scraper instance
func newScraper(cfg *Config, settings receiver.Settings, nvmeUtil nvme.DeviceInfoProvider, deviceSet collections.Set[string]) *nvmeScraper {
	return &nvmeScraper{
		logger:           settings.TelemetrySettings.Logger,
		mb:               metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
		nvmeUtil:         nvmeUtil,
		metadataProvider: nil, // Will be initialized lazily in scrape()
		deviceSet:        deviceSet,

		// Initialize performance optimization components
		deviceTypeCache: newDeviceTypeCache(),
		bufferPool:      newBufferPool(),
		lastScrapeTime:  time.Now(),
		scrapeCount:     0,
	}
}

// setMetadataProvider sets the metadata provider for testing purposes
func (s *nvmeScraper) setMetadataProvider(provider ec2metadataprovider.MetadataProvider) {
	s.metadataProvider = provider
}

// start initializes the scraper
func (s *nvmeScraper) start(_ context.Context, _ component.Host) error {
	s.logger.Debug("Starting unified NVMe scraper", zap.String("receiver", metadata.Type.String()))
	return nil
}

// shutdown cleans up scraper resources
func (s *nvmeScraper) shutdown(_ context.Context) error {
	s.logger.Debug("Shutting down unified NVMe scraper", zap.String("receiver", metadata.Type.String()))
	return nil
}

// scrape performs the main scraping logic with device type routing
func (s *nvmeScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	scrapeStartTime := time.Now()
	scrapeCount := atomic.AddInt64(&s.scrapeCount, 1)

	s.logger.Debug("Began scraping for unified NVMe metrics",
		zap.Int64("scrapeCount", scrapeCount),
		zap.Duration("timeSinceLastScrape", scrapeStartTime.Sub(s.lastScrapeTime)))

	// Check for platform support early
	if !s.isPlatformSupported() {
		s.logger.Debug("NVMe metrics collection is not supported on this platform")
		return s.mb.Emit(), nil
	}

	// Periodically clean up expired cache entries (every 10 scrapes)
	if scrapeCount%10 == 0 {
		s.deviceTypeCache.clear()
		s.logger.Debug("cleaned up expired device type cache entries",
			zap.Int64("scrapeCount", scrapeCount))
	}

	// Discover and group devices by controller with enhanced error handling
	devicesByController, err := s.getDevicesByController()
	if err != nil {
		// Classify error types for better logging and recovery
		if s.isRecoverableError(err) {
			s.logger.Warn("temporary failure during device discovery, will retry on next scrape",
				zap.Error(err),
				zap.String("errorType", s.classifyError(err)))
			return s.mb.Emit(), nil // Return empty metrics but don't fail the scraper
		}
		s.logger.Error("failed to get devices by controller",
			zap.Error(err),
			zap.String("errorType", s.classifyError(err)))
		return pmetric.NewMetrics(), err
	}

	if len(devicesByController) == 0 {
		s.logger.Debug("no NVMe devices found for monitoring")
		return s.mb.Emit(), nil
	}

	// Initialize metadata provider lazily to avoid blocking during tests
	if s.metadataProvider == nil {
		mdCredentialConfig := &configaws.CredentialConfig{}
		s.metadataProvider = ec2metadataprovider.NewMetadataProvider(mdCredentialConfig.Credentials(), retryer.GetDefaultRetryNumber())
	}

	// Get InstanceId from EC2 metadata service with enhanced error handling
	instanceID, err := s.getInstanceIDWithFallback(ctx)
	if err != nil {
		s.logger.Warn("unable to get instance ID from metadata service, using placeholder",
			zap.Error(err),
			zap.String("fallbackValue", instanceID))
	}

	now := pcommon.NewTimestampFromTime(time.Now())
	successfulDevices := 0
	totalDevices := len(devicesByController)
	errorsByType := make(map[string]int)

	// Process each device group with comprehensive error tracking
	for controllerID, devices := range devicesByController {
		// Some devices are owned by root:root, root:disk, etc, so the agent will attempt to
		// retrieve the metric for a device (grouped by controller ID) until the first success
		foundWorkingDevice := false
		var lastError error
		deviceAttempts := 0

		for _, deviceName := range devices.deviceNames {
			if foundWorkingDevice {
				break
			}
			deviceAttempts++

			devicePath, err := s.nvmeUtil.DevicePath(deviceName)
			if err != nil {
				errorType := s.classifyError(err)
				errorsByType[errorType]++
				s.logger.Debug("unable to get device path",
					zap.String("device", deviceName),
					zap.String("errorType", errorType),
					zap.Int("attempt", deviceAttempts),
					zap.Error(err))
				lastError = err
				continue
			}

			// Route to appropriate parsing function based on device type with enhanced error handling
			switch devices.deviceType {
			case "ebs":
				if err := s.processEBSDeviceWithRecovery(devicePath, devices, instanceID, now); err != nil {
					errorType := s.classifyError(err)
					errorsByType[errorType]++
					s.logger.Debug("unable to process EBS device",
						zap.String("device", deviceName),
						zap.String("devicePath", devicePath),
						zap.String("errorType", errorType),
						zap.Int("attempt", deviceAttempts),
						zap.Error(err))
					lastError = err
					continue
				}
			case "instance_store":
				if err := s.processInstanceStoreDeviceWithRecovery(devicePath, devices, instanceID, now); err != nil {
					errorType := s.classifyError(err)
					errorsByType[errorType]++
					s.logger.Debug("unable to process Instance Store device",
						zap.String("device", deviceName),
						zap.String("devicePath", devicePath),
						zap.String("errorType", errorType),
						zap.Int("attempt", deviceAttempts),
						zap.Error(err))
					lastError = err
					continue
				}
			default:
				lastError = fmt.Errorf("unknown device type: %s", devices.deviceType)
				errorsByType["unknown_device_type"]++
				s.logger.Error("unknown device type detected",
					zap.String("device", deviceName),
					zap.String("deviceType", devices.deviceType),
					zap.Int("controllerID", controllerID))
				continue
			}

			foundWorkingDevice = true
			s.logger.Debug("successfully processed device",
				zap.String("device", deviceName),
				zap.String("devicePath", devicePath),
				zap.String("deviceType", devices.deviceType),
				zap.Int("controllerID", controllerID),
				zap.Int("attempts", deviceAttempts))
		}

		if foundWorkingDevice {
			successfulDevices++
		} else {
			errorType := s.classifyError(lastError)
			s.logger.Warn("failed to get metrics for device controller after all attempts",
				zap.Int("controllerID", controllerID),
				zap.String("deviceType", devices.deviceType),
				zap.String("serialNumber", devices.serialNumber),
				zap.Strings("deviceNames", devices.deviceNames),
				zap.Int("totalAttempts", deviceAttempts),
				zap.String("errorType", errorType),
				zap.Error(lastError))
		}
	}

	// Log comprehensive error summary
	if len(errorsByType) > 0 {
		s.logger.Info("error summary for NVMe metrics collection",
			zap.Int("successfulDevices", successfulDevices),
			zap.Int("totalDevices", totalDevices),
			zap.Any("errorsByType", errorsByType))
	}

	// Calculate performance metrics
	scrapeLatency := time.Since(scrapeStartTime)
	s.lastScrapeTime = scrapeStartTime

	s.logger.Debug("completed unified NVMe metrics scraping",
		zap.Int("successfulDevices", successfulDevices),
		zap.Int("totalDevices", totalDevices),
		zap.Float64("successRate", float64(successfulDevices)/float64(totalDevices)*100),
		zap.Duration("scrapeLatency", scrapeLatency),
		zap.Int64("scrapeCount", scrapeCount))

	// Log performance warnings if requirements are not met
	if scrapeLatency > 50*time.Millisecond {
		s.logger.Warn("scrape latency exceeds target requirement",
			zap.Duration("actualLatency", scrapeLatency),
			zap.Duration("targetLatency", 50*time.Millisecond),
			zap.Int("totalDevices", totalDevices))
	}

	if successfulDevices == 0 && totalDevices > 0 {
		s.logger.Warn("no NVMe devices were successfully scraped",
			zap.Int("totalDevices", totalDevices),
			zap.Any("errorsByType", errorsByType))
	}

	return s.mb.Emit(), nil
}

// getDevicesByController discovers and groups devices by controller ID with optimized device type detection
func (s *nvmeScraper) getDevicesByController() (devicesByController, error) {
	discoveryStartTime := time.Now()

	allNvmeDevices, err := s.nvmeUtil.GetAllDevices()
	if err != nil {
		// Enhanced error context for device discovery failures
		if strings.Contains(err.Error(), "only supported on Linux") {
			return nil, fmt.Errorf("platform not supported: %w", err)
		}
		if strings.Contains(err.Error(), "permission denied") {
			return nil, fmt.Errorf("insufficient permissions to access NVMe devices: %w", err)
		}
		return nil, fmt.Errorf("failed to discover NVMe devices: %w", err)
	}

	if len(allNvmeDevices) == 0 {
		s.logger.Debug("no NVMe devices found on system")
		return make(devicesByController), nil
	}

	devices := make(devicesByController)
	processedDevices := 0
	skippedDevices := 0
	errorsByType := make(map[string]int)
	detectionFailures := make([]string, 0)
	cacheHits := 0
	cacheMisses := 0

	// Pre-filter devices to avoid unnecessary processing
	hasAsterisk := s.deviceSet.Contains("*")
	filteredDevices := make([]nvme.DeviceFileAttributes, 0, len(allNvmeDevices))

	for _, device := range allNvmeDevices {
		deviceName := device.DeviceName()
		processedDevices++

		// Check if all devices should be collected. Otherwise check if defined by user
		if !hasAsterisk {
			if isAllowed := s.deviceSet.Contains(deviceName); !isAllowed {
				s.logger.Debug("skipping device not in allowed list",
					zap.String("device", deviceName),
					zap.Int("controllerID", device.Controller()))
				skippedDevices++
				continue
			}
		}

		filteredDevices = append(filteredDevices, device)
	}

	// Group devices by controller first to optimize processing
	devicesByControllerID := make(map[int][]nvme.DeviceFileAttributes)
	for _, device := range filteredDevices {
		controllerID := device.Controller()
		devicesByControllerID[controllerID] = append(devicesByControllerID[controllerID], device)
	}

	// Process each controller group
	for controllerID, controllerDevices := range devicesByControllerID {
		// Use the first device in the controller group for type detection and serial retrieval
		primaryDevice := controllerDevices[0]
		deviceName := primaryDevice.DeviceName()

		// Detect device type using optimized detection logic with caching
		cacheKey := fmt.Sprintf("controller-%d-namespace-%d", primaryDevice.Controller(), primaryDevice.Namespace())
		var deviceType string
		var detectionErr error

		if cachedType, found := s.deviceTypeCache.get(cacheKey); found {
			deviceType = cachedType
			cacheHits++
		} else {
			deviceType, detectionErr = s.detectDeviceTypeWithRetry(&primaryDevice)
			cacheMisses++
		}

		if detectionErr != nil {
			errorType := s.classifyError(detectionErr)
			errorsByType[errorType]++

			// Add all devices in this controller to the failure list
			for _, device := range controllerDevices {
				detectionFailures = append(detectionFailures, device.DeviceName())
			}

			s.logger.Debug("failed to detect device type for controller",
				zap.Int("controllerID", controllerID),
				zap.String("primaryDevice", deviceName),
				zap.Int("devicesInController", len(controllerDevices)),
				zap.String("errorType", errorType),
				zap.Error(detectionErr))
			continue
		}

		// Get device serial number with enhanced error handling and fallback
		serial, err := s.getDeviceSerialWithFallback(&primaryDevice, deviceType)
		if err != nil {
			errorType := s.classifyError(err)
			s.logger.Warn("unable to get serial number for controller, using fallback",
				zap.Int("controllerID", controllerID),
				zap.String("primaryDevice", deviceName),
				zap.String("deviceType", deviceType),
				zap.String("errorType", errorType),
				zap.String("fallbackSerial", serial),
				zap.Error(err))
		}

		// For EBS devices, format the serial as volume ID with validation
		if deviceType == "ebs" {
			serial = s.formatEBSSerial(serial, deviceName)
		}

		// Collect all device names for this controller
		deviceNames := make([]string, len(controllerDevices))
		for i, device := range controllerDevices {
			deviceNames[i] = device.DeviceName()
		}

		devices[controllerID] = &nvmeDevices{
			deviceType:   deviceType,
			serialNumber: serial,
			deviceNames:  deviceNames,
		}

		s.logger.Debug("discovered controller group",
			zap.Int("controllerID", controllerID),
			zap.String("deviceType", deviceType),
			zap.String("serialNumber", serial),
			zap.Strings("deviceNames", deviceNames),
			zap.Int("deviceCount", len(deviceNames)))
	}

	// Enhanced logging for device discovery results
	totalErrors := 0
	for _, count := range errorsByType {
		totalErrors += count
	}

	discoveryLatency := time.Since(discoveryStartTime)

	s.logger.Debug("completed optimized device discovery",
		zap.Int("totalProcessed", processedDevices),
		zap.Int("filteredDevices", len(filteredDevices)),
		zap.Int("discoveredControllers", len(devices)),
		zap.Int("skippedDevices", skippedDevices),
		zap.Int("totalErrors", totalErrors),
		zap.Int("cacheHits", cacheHits),
		zap.Int("cacheMisses", cacheMisses),
		zap.Duration("discoveryLatency", discoveryLatency),
		zap.Any("errorsByType", errorsByType))

	if len(detectionFailures) > 0 {
		s.logger.Info("device type detection failures",
			zap.Strings("failedDevices", detectionFailures),
			zap.Any("errorsByType", errorsByType))
	}

	if len(devices) == 0 && totalErrors > 0 {
		return devices, fmt.Errorf("no devices found, encountered %d errors during discovery (by type: %v)", totalErrors, errorsByType)
	}

	return devices, nil
}

// detectDeviceTypeWithRetry attempts device type detection with caching and retry logic for recoverable errors
func (s *nvmeScraper) detectDeviceTypeWithRetry(device *nvme.DeviceFileAttributes) (string, error) {
	deviceName := device.DeviceName()

	// Create cache key based on device controller and namespace for uniqueness
	cacheKey := fmt.Sprintf("controller-%d-namespace-%d", device.Controller(), device.Namespace())

	// Check cache first - use longer TTL for successful detections
	if cachedType, found := s.deviceTypeCache.get(cacheKey); found {
		s.logger.Debug("using cached device type",
			zap.String("device", deviceName),
			zap.String("deviceType", cachedType),
			zap.String("cacheKey", cacheKey))
		return cachedType, nil
	}

	const maxRetries = 3
	const baseDelay = 100 * time.Millisecond
	const successCacheTTL = 5 * time.Minute  // Cache successful detections longer
	const failureCacheTTL = 30 * time.Second // Cache failures for shorter time

	var lastError error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		deviceType, err := s.nvmeUtil.DetectDeviceType(device)
		if err == nil {
			// Cache successful detection
			s.deviceTypeCache.set(cacheKey, deviceType, successCacheTTL)

			if attempt > 1 {
				s.logger.Debug("device type detection succeeded after retry",
					zap.String("device", deviceName),
					zap.String("deviceType", deviceType),
					zap.Int("attempt", attempt),
					zap.String("cacheKey", cacheKey))
			}
			return deviceType, nil
		}

		lastError = err

		// Check if this is a recoverable error worth retrying
		if !s.isRecoverableError(err) {
			s.logger.Debug("device type detection failed with non-recoverable error",
				zap.String("device", deviceName),
				zap.String("errorType", s.classifyError(err)),
				zap.Int("attempt", attempt),
				zap.Error(err))
			break
		}

		// Don't retry on the last attempt
		if attempt < maxRetries {
			delay := time.Duration(attempt) * baseDelay
			s.logger.Debug("device type detection failed, retrying",
				zap.String("device", deviceName),
				zap.String("errorType", s.classifyError(err)),
				zap.Int("attempt", attempt),
				zap.Duration("retryDelay", delay),
				zap.Error(err))

			time.Sleep(delay)
		}
	}

	// Cache the failure for a shorter time to avoid repeated expensive operations
	// but still allow for recovery on subsequent scrapes
	finalError := fmt.Errorf("device type detection failed for %s after %d attempts: %w", deviceName, maxRetries, lastError)

	// Don't cache permanent errors (like platform unsupported)
	if s.isRecoverableError(lastError) {
		s.deviceTypeCache.set(cacheKey, "", failureCacheTTL)
	}

	return "", finalError
}

// getDeviceSerialWithFallback gets device serial with fallback handling
func (s *nvmeScraper) getDeviceSerialWithFallback(device *nvme.DeviceFileAttributes, deviceType string) (string, error) {
	serial, err := s.nvmeUtil.GetDeviceSerial(device)
	if err != nil {
		// Generate a fallback serial number that's still useful for identification
		fallbackSerial := fmt.Sprintf("unknown-%s-controller-%d", deviceType, device.Controller())
		return fallbackSerial, fmt.Errorf("serial retrieval failed, using fallback: %w", err)
	}
	return serial, nil
}

// formatEBSSerial formats EBS device serial numbers with validation
func (s *nvmeScraper) formatEBSSerial(serial, deviceName string) string {
	// The serial should begin with vol and have content after the vol prefix
	if strings.HasPrefix(serial, "vol") && len(serial) > 3 {
		return fmt.Sprintf("vol-%s", serial[3:])
	}

	// If it doesn't match expected format, log and continue with original
	if !strings.HasPrefix(serial, "unknown-") {
		s.logger.Debug("EBS device serial is not in expected volume ID format",
			zap.String("device", deviceName),
			zap.String("serial", serial),
			zap.String("expectedFormat", "vol-*"))
	}

	return serial
}

// recordMetric safely records a metric value with overflow protection and prefix application
func (s *nvmeScraper) recordMetric(recordFn recordDataMetricFunc, ts pcommon.Timestamp, val uint64) {
	converted, err := nvme.SafeUint64ToInt64(val)
	if err != nil {
		s.logger.Debug("skipping metric due to potential integer overflow",
			zap.Uint64("value", val),
			zap.Error(err))
		return
	}
	recordFn(ts, converted)
}

// isPlatformSupported checks if the current platform supports NVMe operations
func (s *nvmeScraper) isPlatformSupported() bool {
	// Try a simple device discovery to check platform support
	_, err := s.nvmeUtil.GetAllDevices()
	if err != nil {
		// Check if this is a platform support error
		if err.Error() == "nvme device discovery is only supported on Linux" ||
			err.Error() == "nvme device operations are only supported on Linux" {
			return false
		}
	}
	return true
}

// isRecoverableError determines if an error is recoverable and should be retried
func (s *nvmeScraper) isRecoverableError(err error) bool {
	if err == nil {
		return false
	}

	errorMsg := err.Error()

	// Temporary permission issues that might resolve
	if strings.Contains(errorMsg, "permission denied") ||
		strings.Contains(errorMsg, "insufficient permissions") {
		return true
	}

	// Device busy or temporarily unavailable
	if strings.Contains(errorMsg, "device or resource busy") ||
		strings.Contains(errorMsg, "temporarily unavailable") {
		return true
	}

	// I/O errors that might be temporary
	if strings.Contains(errorMsg, "I/O error") ||
		strings.Contains(errorMsg, "input/output error") {
		return true
	}

	// Network-related errors for metadata service
	if strings.Contains(errorMsg, "connection refused") ||
		strings.Contains(errorMsg, "timeout") ||
		strings.Contains(errorMsg, "network unreachable") {
		return true
	}

	return false
}

// classifyError categorizes errors for better logging and monitoring
func (s *nvmeScraper) classifyError(err error) string {
	if err == nil {
		return "none"
	}

	errorMsg := strings.ToLower(err.Error())

	// Platform support errors
	if strings.Contains(errorMsg, "only supported on linux") ||
		strings.Contains(errorMsg, "unsupported platform") {
		return "platform_unsupported"
	}

	// Permission errors
	if strings.Contains(errorMsg, "permission denied") ||
		strings.Contains(errorMsg, "insufficient permissions") ||
		strings.Contains(errorMsg, "cap_sys_admin") {
		return "permission_denied"
	}

	// Device access errors
	if strings.Contains(errorMsg, "device not found") ||
		strings.Contains(errorMsg, "no such file or directory") {
		return "device_not_found"
	}

	if strings.Contains(errorMsg, "device or resource busy") {
		return "device_busy"
	}

	// ioctl operation errors
	if strings.Contains(errorMsg, "ioctl") {
		return "ioctl_failed"
	}

	// Parsing errors
	if strings.Contains(errorMsg, "invalid magic number") ||
		strings.Contains(errorMsg, "magic") {
		return "invalid_magic_number"
	}

	if strings.Contains(errorMsg, "insufficient data") ||
		strings.Contains(errorMsg, "buffer overflow") {
		return "data_parsing_error"
	}

	// Device type detection errors
	if strings.Contains(errorMsg, "unknown device type") ||
		strings.Contains(errorMsg, "detection failed") {
		return "device_type_detection_failed"
	}

	// Metadata service errors
	if strings.Contains(errorMsg, "metadata service") ||
		strings.Contains(errorMsg, "instance id") {
		return "metadata_service_error"
	}

	// I/O errors
	if strings.Contains(errorMsg, "i/o error") ||
		strings.Contains(errorMsg, "input/output error") {
		return "io_error"
	}

	// Network errors
	if strings.Contains(errorMsg, "connection refused") ||
		strings.Contains(errorMsg, "timeout") ||
		strings.Contains(errorMsg, "network unreachable") {
		return "network_error"
	}

	// Overflow errors
	if strings.Contains(errorMsg, "overflow") ||
		strings.Contains(errorMsg, "too large") {
		return "overflow_error"
	}

	return "unknown_error"
}

// getInstanceIDWithFallback retrieves instance ID with fallback handling
func (s *nvmeScraper) getInstanceIDWithFallback(ctx context.Context) (string, error) {
	instanceID, err := s.metadataProvider.InstanceID(ctx)
	if err != nil {
		// Try to provide a more informative fallback
		fallbackID := "unknown"

		// Check if we can determine any identifying information
		if hostname, hostErr := os.Hostname(); hostErr == nil && hostname != "" {
			fallbackID = fmt.Sprintf("unknown-%s", hostname)
		}

		return fallbackID, fmt.Errorf("metadata service unavailable: %w", err)
	}
	return instanceID, nil
}

// processEBSDeviceWithRecovery processes an EBS device with enhanced error handling and recovery
func (s *nvmeScraper) processEBSDeviceWithRecovery(devicePath string, devices *nvmeDevices, instanceID string, now pcommon.Timestamp) error {
	const maxRetries = 2
	const baseDelay = 50 * time.Millisecond

	var lastError error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Get EBS metrics from the device with detailed error context
		metrics, err := nvme.GetEBSMetrics(devicePath)
		if err != nil {
			lastError = fmt.Errorf("failed to get EBS metrics from device %s (controller: %s, serial: %s): %w",
				devicePath, devices.deviceType, devices.serialNumber, err)

			// Check if this is a recoverable error worth retrying
			if !s.isRecoverableError(err) || attempt == maxRetries {
				s.logger.Debug("EBS metrics retrieval failed",
					zap.String("device", devicePath),
					zap.String("errorType", s.classifyError(err)),
					zap.Int("attempt", attempt),
					zap.Bool("recoverable", s.isRecoverableError(err)),
					zap.Error(err))
				return lastError
			}

			delay := time.Duration(attempt) * baseDelay
			s.logger.Debug("EBS metrics retrieval failed, retrying",
				zap.String("device", devicePath),
				zap.String("errorType", s.classifyError(err)),
				zap.Int("attempt", attempt),
				zap.Duration("retryDelay", delay),
				zap.Error(err))

			time.Sleep(delay)
			continue
		}

		// Validate metrics before recording to detect potential corruption
		if err := s.validateEBSMetrics(&metrics, devicePath); err != nil {
			s.logger.Warn("EBS metrics validation failed, but continuing with available data",
				zap.String("device", devicePath),
				zap.Error(err))
			// Continue processing despite validation warnings
		}

		// Create resource builder and set dimensions
		rb := s.mb.NewResourceBuilder()
		rb.SetInstanceID(instanceID)
		rb.SetDeviceType("ebs")
		rb.SetDevice(devicePath)
		rb.SetSerialNumber(devices.serialNumber)

		// Record all EBS metrics with safe conversion and overflow protection
		metricsRecorded := 0
		totalMetrics := 11

		s.recordMetricWithContextAndCount("diskio_ebs_total_read_ops", s.mb.RecordDiskioEbsTotalReadOpsDataPoint, now, metrics.ReadOps, devicePath, &metricsRecorded)
		s.recordMetricWithContextAndCount("diskio_ebs_total_write_ops", s.mb.RecordDiskioEbsTotalWriteOpsDataPoint, now, metrics.WriteOps, devicePath, &metricsRecorded)
		s.recordMetricWithContextAndCount("diskio_ebs_total_read_bytes", s.mb.RecordDiskioEbsTotalReadBytesDataPoint, now, metrics.ReadBytes, devicePath, &metricsRecorded)
		s.recordMetricWithContextAndCount("diskio_ebs_total_write_bytes", s.mb.RecordDiskioEbsTotalWriteBytesDataPoint, now, metrics.WriteBytes, devicePath, &metricsRecorded)
		s.recordMetricWithContextAndCount("diskio_ebs_total_read_time", s.mb.RecordDiskioEbsTotalReadTimeDataPoint, now, metrics.TotalReadTime, devicePath, &metricsRecorded)
		s.recordMetricWithContextAndCount("diskio_ebs_total_write_time", s.mb.RecordDiskioEbsTotalWriteTimeDataPoint, now, metrics.TotalWriteTime, devicePath, &metricsRecorded)
		s.recordMetricWithContextAndCount("diskio_ebs_volume_performance_exceeded_iops", s.mb.RecordDiskioEbsVolumePerformanceExceededIopsDataPoint, now, metrics.EBSIOPSExceeded, devicePath, &metricsRecorded)
		s.recordMetricWithContextAndCount("diskio_ebs_volume_performance_exceeded_tp", s.mb.RecordDiskioEbsVolumePerformanceExceededTpDataPoint, now, metrics.EBSThroughputExceeded, devicePath, &metricsRecorded)
		s.recordMetricWithContextAndCount("diskio_ebs_ec2_instance_performance_exceeded_iops", s.mb.RecordDiskioEbsEc2InstancePerformanceExceededIopsDataPoint, now, metrics.EC2IOPSExceeded, devicePath, &metricsRecorded)
		s.recordMetricWithContextAndCount("diskio_ebs_ec2_instance_performance_exceeded_tp", s.mb.RecordDiskioEbsEc2InstancePerformanceExceededTpDataPoint, now, metrics.EC2ThroughputExceeded, devicePath, &metricsRecorded)
		s.recordMetricWithContextAndCount("diskio_ebs_volume_queue_length", s.mb.RecordDiskioEbsVolumeQueueLengthDataPoint, now, metrics.QueueLength, devicePath, &metricsRecorded)

		// Log metrics recording summary
		s.logger.Debug("EBS metrics recorded successfully",
			zap.String("device", devicePath),
			zap.Int("metricsRecorded", metricsRecorded),
			zap.Int("totalMetrics", totalMetrics),
			zap.Int("attempt", attempt))

		// Emit metrics for this resource
		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))

		return nil
	}

	return lastError
}

// processInstanceStoreDeviceWithRecovery processes an Instance Store device with enhanced error handling and recovery
func (s *nvmeScraper) processInstanceStoreDeviceWithRecovery(devicePath string, devices *nvmeDevices, instanceID string, now pcommon.Timestamp) error {
	const maxRetries = 2
	const baseDelay = 50 * time.Millisecond

	var lastError error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Get Instance Store metrics from the device with detailed error context
		metrics, err := nvme.GetInstanceStoreMetrics(devicePath)
		if err != nil {
			lastError = fmt.Errorf("failed to get Instance Store metrics from device %s (controller: %s, serial: %s): %w",
				devicePath, devices.deviceType, devices.serialNumber, err)

			// Check if this is a recoverable error worth retrying
			if !s.isRecoverableError(err) || attempt == maxRetries {
				s.logger.Debug("Instance Store metrics retrieval failed",
					zap.String("device", devicePath),
					zap.String("errorType", s.classifyError(err)),
					zap.Int("attempt", attempt),
					zap.Bool("recoverable", s.isRecoverableError(err)),
					zap.Error(err))
				return lastError
			}

			delay := time.Duration(attempt) * baseDelay
			s.logger.Debug("Instance Store metrics retrieval failed, retrying",
				zap.String("device", devicePath),
				zap.String("errorType", s.classifyError(err)),
				zap.Int("attempt", attempt),
				zap.Duration("retryDelay", delay),
				zap.Error(err))

			time.Sleep(delay)
			continue
		}

		// Validate metrics before recording to detect potential corruption
		if err := s.validateInstanceStoreMetrics(&metrics, devicePath); err != nil {
			s.logger.Warn("Instance Store metrics validation failed, but continuing with available data",
				zap.String("device", devicePath),
				zap.Error(err))
			// Continue processing despite validation warnings
		}

		// Create resource builder and set dimensions
		rb := s.mb.NewResourceBuilder()
		rb.SetInstanceID(instanceID)
		rb.SetDeviceType("instance_store")
		rb.SetDevice(devicePath)
		rb.SetSerialNumber(devices.serialNumber)

		// Record all Instance Store metrics with safe conversion and overflow protection
		// Note: Instance Store devices skip EBS-specific fields (EBSIOPSExceeded, EBSThroughputExceeded)
		metricsRecorded := 0
		totalMetrics := 9

		s.recordMetricWithContextAndCount("diskio_instance_store_total_read_ops", s.mb.RecordDiskioInstanceStoreTotalReadOpsDataPoint, now, metrics.ReadOps, devicePath, &metricsRecorded)
		s.recordMetricWithContextAndCount("diskio_instance_store_total_write_ops", s.mb.RecordDiskioInstanceStoreTotalWriteOpsDataPoint, now, metrics.WriteOps, devicePath, &metricsRecorded)
		s.recordMetricWithContextAndCount("diskio_instance_store_total_read_bytes", s.mb.RecordDiskioInstanceStoreTotalReadBytesDataPoint, now, metrics.ReadBytes, devicePath, &metricsRecorded)
		s.recordMetricWithContextAndCount("diskio_instance_store_total_write_bytes", s.mb.RecordDiskioInstanceStoreTotalWriteBytesDataPoint, now, metrics.WriteBytes, devicePath, &metricsRecorded)
		s.recordMetricWithContextAndCount("diskio_instance_store_total_read_time", s.mb.RecordDiskioInstanceStoreTotalReadTimeDataPoint, now, metrics.TotalReadTime, devicePath, &metricsRecorded)
		s.recordMetricWithContextAndCount("diskio_instance_store_total_write_time", s.mb.RecordDiskioInstanceStoreTotalWriteTimeDataPoint, now, metrics.TotalWriteTime, devicePath, &metricsRecorded)
		s.recordMetricWithContextAndCount("diskio_instance_store_volume_performance_exceeded_iops", s.mb.RecordDiskioInstanceStoreVolumePerformanceExceededIopsDataPoint, now, metrics.EC2IOPSExceeded, devicePath, &metricsRecorded)
		s.recordMetricWithContextAndCount("diskio_instance_store_volume_performance_exceeded_tp", s.mb.RecordDiskioInstanceStoreVolumePerformanceExceededTpDataPoint, now, metrics.EC2ThroughputExceeded, devicePath, &metricsRecorded)
		s.recordMetricWithContextAndCount("diskio_instance_store_volume_queue_length", s.mb.RecordDiskioInstanceStoreVolumeQueueLengthDataPoint, now, metrics.QueueLength, devicePath, &metricsRecorded)

		// Log metrics recording summary
		s.logger.Debug("Instance Store metrics recorded successfully",
			zap.String("device", devicePath),
			zap.Int("metricsRecorded", metricsRecorded),
			zap.Int("totalMetrics", totalMetrics),
			zap.Int("attempt", attempt))

		// Emit metrics for this resource
		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))

		return nil
	}

	return lastError
}

// recordMetricWithContext records a metric with additional context for debugging
func (s *nvmeScraper) recordMetricWithContext(metricName string, recordFn recordDataMetricFunc, ts pcommon.Timestamp, val uint64, devicePath string) {
	converted, err := nvme.SafeUint64ToInt64(val)
	if err != nil {
		s.logger.Debug("skipping metric due to potential integer overflow",
			zap.String("metric", metricName),
			zap.String("device", devicePath),
			zap.Uint64("value", val),
			zap.Error(err))
		return
	}
	recordFn(ts, converted)
}

// recordMetricWithContextAndCount records a metric with additional context and increments counter
func (s *nvmeScraper) recordMetricWithContextAndCount(metricName string, recordFn recordDataMetricFunc, ts pcommon.Timestamp, val uint64, devicePath string, counter *int) {
	converted, err := nvme.SafeUint64ToInt64(val)
	if err != nil {
		s.logger.Debug("skipping metric due to potential integer overflow",
			zap.String("metric", metricName),
			zap.String("device", devicePath),
			zap.Uint64("value", val),
			zap.Error(err))
		return
	}
	recordFn(ts, converted)
	*counter++
}

// validateEBSMetrics performs comprehensive validation on EBS metrics to detect corruption and security issues
func (s *nvmeScraper) validateEBSMetrics(metrics *nvme.EBSMetrics, devicePath string) error {
	// Security check: Validate magic number to ensure data integrity
	if metrics.EBSMagic != nvme.EBSMagicNumber {
		s.logger.Error("EBS metrics validation failed: invalid magic number",
			zap.String("device", devicePath),
			zap.Uint64("expectedMagic", nvme.EBSMagicNumber),
			zap.Uint64("actualMagic", metrics.EBSMagic))
		return fmt.Errorf("invalid EBS magic number: expected 0x%X, got 0x%X", nvme.EBSMagicNumber, metrics.EBSMagic)
	}

	// Check for obviously invalid values that might indicate corruption
	if metrics.ReadOps == 0 && metrics.WriteOps == 0 && metrics.ReadBytes == 0 && metrics.WriteBytes == 0 {
		s.logger.Debug("EBS device shows no activity, this may be normal for unused devices",
			zap.String("device", devicePath))
	}

	// Security check: Validate metric bounds to detect potential attacks or corruption
	const (
		maxReasonableOps      = uint64(1e12) // 1 trillion operations
		maxReasonableBytes    = uint64(1e18) // 1 exabyte
		maxReasonableTime     = uint64(1e18) // ~31 years in nanoseconds
		maxReasonableExceeded = uint64(1e12) // 1 trillion exceeded events
		maxReasonableQueueLen = uint64(1e6)  // 1 million queue length
	)

	// Validate operation counters for potential overflow attacks
	if metrics.ReadOps > maxReasonableOps {
		s.logger.Warn("EBS device ReadOps value exceeds reasonable bounds, possible data corruption or attack",
			zap.String("device", devicePath),
			zap.Uint64("readOps", metrics.ReadOps),
			zap.Uint64("maxReasonable", maxReasonableOps))
	}
	if metrics.WriteOps > maxReasonableOps {
		s.logger.Warn("EBS device WriteOps value exceeds reasonable bounds, possible data corruption or attack",
			zap.String("device", devicePath),
			zap.Uint64("writeOps", metrics.WriteOps),
			zap.Uint64("maxReasonable", maxReasonableOps))
	}

	// Validate byte counters for potential overflow attacks
	if metrics.ReadBytes > maxReasonableBytes {
		s.logger.Warn("EBS device ReadBytes value exceeds reasonable bounds, possible data corruption or attack",
			zap.String("device", devicePath),
			zap.Uint64("readBytes", metrics.ReadBytes),
			zap.Uint64("maxReasonable", maxReasonableBytes))
	}
	if metrics.WriteBytes > maxReasonableBytes {
		s.logger.Warn("EBS device WriteBytes value exceeds reasonable bounds, possible data corruption or attack",
			zap.String("device", devicePath),
			zap.Uint64("writeBytes", metrics.WriteBytes),
			zap.Uint64("maxReasonable", maxReasonableBytes))
	}

	// Validate time counters for potential overflow attacks
	if metrics.TotalReadTime > maxReasonableTime {
		s.logger.Warn("EBS device TotalReadTime value exceeds reasonable bounds, possible data corruption or attack",
			zap.String("device", devicePath),
			zap.Uint64("totalReadTime", metrics.TotalReadTime),
			zap.Uint64("maxReasonable", maxReasonableTime))
	}
	if metrics.TotalWriteTime > maxReasonableTime {
		s.logger.Warn("EBS device TotalWriteTime value exceeds reasonable bounds, possible data corruption or attack",
			zap.String("device", devicePath),
			zap.Uint64("totalWriteTime", metrics.TotalWriteTime),
			zap.Uint64("maxReasonable", maxReasonableTime))
	}

	// Validate queue length for potential overflow attacks
	if metrics.QueueLength > maxReasonableQueueLen {
		s.logger.Warn("EBS device QueueLength value exceeds reasonable bounds, possible data corruption or attack",
			zap.String("device", devicePath),
			zap.Uint64("queueLength", metrics.QueueLength),
			zap.Uint64("maxReasonable", maxReasonableQueueLen))
	}

	// Check for impossible relationships that might indicate data corruption or manipulation
	if metrics.ReadBytes > 0 && metrics.ReadOps == 0 {
		s.logger.Warn("EBS device shows read bytes but no read operations, possible data corruption",
			zap.String("device", devicePath),
			zap.Uint64("readBytes", metrics.ReadBytes),
			zap.Uint64("readOps", metrics.ReadOps))
	}

	if metrics.WriteBytes > 0 && metrics.WriteOps == 0 {
		s.logger.Warn("EBS device shows write bytes but no write operations, possible data corruption",
			zap.String("device", devicePath),
			zap.Uint64("writeBytes", metrics.WriteBytes),
			zap.Uint64("writeOps", metrics.WriteOps))
	}

	// Security check: Validate that time values are reasonable relative to operations
	if metrics.ReadOps > 0 {
		avgReadTime := metrics.TotalReadTime / metrics.ReadOps
		if avgReadTime > 1e12 { // More than 1 second per operation is suspicious
			s.logger.Warn("EBS device shows unusually high average read time, possible data corruption",
				zap.String("device", devicePath),
				zap.Uint64("avgReadTimeNs", avgReadTime),
				zap.Uint64("readOps", metrics.ReadOps),
				zap.Uint64("totalReadTime", metrics.TotalReadTime))
		}
	}

	if metrics.WriteOps > 0 {
		avgWriteTime := metrics.TotalWriteTime / metrics.WriteOps
		if avgWriteTime > 1e12 { // More than 1 second per operation is suspicious
			s.logger.Warn("EBS device shows unusually high average write time, possible data corruption",
				zap.String("device", devicePath),
				zap.Uint64("avgWriteTimeNs", avgWriteTime),
				zap.Uint64("writeOps", metrics.WriteOps),
				zap.Uint64("totalWriteTime", metrics.TotalWriteTime))
		}
	}

	return nil
}

// validateInstanceStoreMetrics performs comprehensive validation on Instance Store metrics to detect corruption and security issues
func (s *nvmeScraper) validateInstanceStoreMetrics(metrics *nvme.InstanceStoreMetrics, devicePath string) error {
	// Security check: Validate magic number to ensure data integrity
	if metrics.Magic != nvme.InstanceStoreMagicNumber {
		s.logger.Error("Instance Store metrics validation failed: invalid magic number",
			zap.String("device", devicePath),
			zap.Uint32("expectedMagic", nvme.InstanceStoreMagicNumber),
			zap.Uint32("actualMagic", metrics.Magic))
		return fmt.Errorf("invalid Instance Store magic number: expected 0x%X, got 0x%X", nvme.InstanceStoreMagicNumber, metrics.Magic)
	}

	// Check for obviously invalid values that might indicate corruption
	if metrics.ReadOps == 0 && metrics.WriteOps == 0 && metrics.ReadBytes == 0 && metrics.WriteBytes == 0 {
		s.logger.Debug("Instance Store device shows no activity, this may be normal for unused devices",
			zap.String("device", devicePath))
	}

	// Security check: Validate metric bounds to detect potential attacks or corruption
	const (
		maxReasonableOps        = uint64(1e12) // 1 trillion operations
		maxReasonableBytes      = uint64(1e18) // 1 exabyte
		maxReasonableTime       = uint64(1e18) // ~31 years in nanoseconds
		maxReasonableExceeded   = uint64(1e12) // 1 trillion exceeded events
		maxReasonableQueueLen   = uint64(1e6)  // 1 million queue length
		maxReasonableHistograms = uint64(10)   // Maximum number of histograms
		maxReasonableBins       = uint64(256)  // Maximum number of bins per histogram
	)

	// Validate operation counters for potential overflow attacks
	if metrics.ReadOps > maxReasonableOps {
		s.logger.Warn("Instance Store device ReadOps value exceeds reasonable bounds, possible data corruption or attack",
			zap.String("device", devicePath),
			zap.Uint64("readOps", metrics.ReadOps),
			zap.Uint64("maxReasonable", maxReasonableOps))
	}
	if metrics.WriteOps > maxReasonableOps {
		s.logger.Warn("Instance Store device WriteOps value exceeds reasonable bounds, possible data corruption or attack",
			zap.String("device", devicePath),
			zap.Uint64("writeOps", metrics.WriteOps),
			zap.Uint64("maxReasonable", maxReasonableOps))
	}

	// Validate byte counters for potential overflow attacks
	if metrics.ReadBytes > maxReasonableBytes {
		s.logger.Warn("Instance Store device ReadBytes value exceeds reasonable bounds, possible data corruption or attack",
			zap.String("device", devicePath),
			zap.Uint64("readBytes", metrics.ReadBytes),
			zap.Uint64("maxReasonable", maxReasonableBytes))
	}
	if metrics.WriteBytes > maxReasonableBytes {
		s.logger.Warn("Instance Store device WriteBytes value exceeds reasonable bounds, possible data corruption or attack",
			zap.String("device", devicePath),
			zap.Uint64("writeBytes", metrics.WriteBytes),
			zap.Uint64("maxReasonable", maxReasonableBytes))
	}

	// Validate time counters for potential overflow attacks
	if metrics.TotalReadTime > maxReasonableTime {
		s.logger.Warn("Instance Store device TotalReadTime value exceeds reasonable bounds, possible data corruption or attack",
			zap.String("device", devicePath),
			zap.Uint64("totalReadTime", metrics.TotalReadTime),
			zap.Uint64("maxReasonable", maxReasonableTime))
	}
	if metrics.TotalWriteTime > maxReasonableTime {
		s.logger.Warn("Instance Store device TotalWriteTime value exceeds reasonable bounds, possible data corruption or attack",
			zap.String("device", devicePath),
			zap.Uint64("totalWriteTime", metrics.TotalWriteTime),
			zap.Uint64("maxReasonable", maxReasonableTime))
	}

	// Validate queue length for potential overflow attacks
	if metrics.QueueLength > maxReasonableQueueLen {
		s.logger.Warn("Instance Store device QueueLength value exceeds reasonable bounds, possible data corruption or attack",
			zap.String("device", devicePath),
			zap.Uint64("queueLength", metrics.QueueLength),
			zap.Uint64("maxReasonable", maxReasonableQueueLen))
	}

	// Validate histogram metadata for potential attacks
	if metrics.NumHistograms > maxReasonableHistograms {
		s.logger.Warn("Instance Store device NumHistograms value exceeds reasonable bounds, possible data corruption or attack",
			zap.String("device", devicePath),
			zap.Uint64("numHistograms", metrics.NumHistograms),
			zap.Uint64("maxReasonable", maxReasonableHistograms))
	}
	if metrics.NumBins > maxReasonableBins {
		s.logger.Warn("Instance Store device NumBins value exceeds reasonable bounds, possible data corruption or attack",
			zap.String("device", devicePath),
			zap.Uint64("numBins", metrics.NumBins),
			zap.Uint64("maxReasonable", maxReasonableBins))
	}

	// Check for impossible relationships that might indicate data corruption or manipulation
	if metrics.ReadBytes > 0 && metrics.ReadOps == 0 {
		s.logger.Warn("Instance Store device shows read bytes but no read operations, possible data corruption",
			zap.String("device", devicePath),
			zap.Uint64("readBytes", metrics.ReadBytes),
			zap.Uint64("readOps", metrics.ReadOps))
	}

	if metrics.WriteBytes > 0 && metrics.WriteOps == 0 {
		s.logger.Warn("Instance Store device shows write bytes but no write operations, possible data corruption",
			zap.String("device", devicePath),
			zap.Uint64("writeBytes", metrics.WriteBytes),
			zap.Uint64("writeOps", metrics.WriteOps))
	}

	// Security check: Validate that time values are reasonable relative to operations
	if metrics.ReadOps > 0 {
		avgReadTime := metrics.TotalReadTime / metrics.ReadOps
		if avgReadTime > 1e12 { // More than 1 second per operation is suspicious
			s.logger.Warn("Instance Store device shows unusually high average read time, possible data corruption",
				zap.String("device", devicePath),
				zap.Uint64("avgReadTimeNs", avgReadTime),
				zap.Uint64("readOps", metrics.ReadOps),
				zap.Uint64("totalReadTime", metrics.TotalReadTime))
		}
	}

	if metrics.WriteOps > 0 {
		avgWriteTime := metrics.TotalWriteTime / metrics.WriteOps
		if avgWriteTime > 1e12 { // More than 1 second per operation is suspicious
			s.logger.Warn("Instance Store device shows unusually high average write time, possible data corruption",
				zap.String("device", devicePath),
				zap.Uint64("avgWriteTimeNs", avgWriteTime),
				zap.Uint64("writeOps", metrics.WriteOps),
				zap.Uint64("totalWriteTime", metrics.TotalWriteTime))
		}
	}

	// Security check: Validate histogram consistency
	if metrics.NumHistograms > 0 && metrics.NumBins == 0 {
		s.logger.Warn("Instance Store device shows histograms but no bins, possible data corruption",
			zap.String("device", devicePath),
			zap.Uint64("numHistograms", metrics.NumHistograms),
			zap.Uint64("numBins", metrics.NumBins))
	}

	return nil
}
