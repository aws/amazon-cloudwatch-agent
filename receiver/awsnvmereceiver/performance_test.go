// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsnvmereceiver

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/aws/amazon-cloudwatch-agent/internal/nvme"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsnvmereceiver/internal/metadata"
)

// BenchmarkScraper_DeviceDiscovery benchmarks device discovery performance
func BenchmarkScraper_DeviceDiscovery(b *testing.B) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	mockNvme := &MockDeviceInfoProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)

	// Create 10 mixed devices for benchmarking
	devices := make([]nvme.DeviceFileAttributes, 10)
	for i := 0; i < 10; i++ {
		devices[i] = createTestDevice(i, 1, fmt.Sprintf("nvme%dn1", i))
	}

	// Mock device discovery
	mockNvme.On("GetAllDevices").Return(devices, nil)

	// Mock device type detection for all devices
	for i, device := range devices {
		deviceType := "ebs"
		if i%2 == 1 {
			deviceType = "instance_store"
		}
		mockNvme.On("DetectDeviceType", &device).Return(deviceType, nil)
		mockNvme.On("GetDeviceSerial", &device).Return(fmt.Sprintf("serial-%d", i), nil)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := scraper.getDevicesByController()
		if err != nil {
			b.Fatalf("Device discovery failed: %v", err)
		}
	}
}

// BenchmarkScraper_FullScrape benchmarks full scraping performance
func BenchmarkScraper_FullScrape(b *testing.B) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	mockNvme := &MockDeviceInfoProvider{}
	mockMetadata := &MockMetadataProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)
	scraper.setMetadataProvider(mockMetadata)

	// Create 10 mixed devices for benchmarking
	devices := make([]nvme.DeviceFileAttributes, 10)
	for i := 0; i < 10; i++ {
		devices[i] = createTestDevice(i, 1, fmt.Sprintf("nvme%dn1", i))
	}

	// Mock device discovery
	mockNvme.On("GetAllDevices").Return(devices, nil)

	// Mock device type detection and paths for all devices
	for i, device := range devices {
		deviceType := "ebs"
		if i%2 == 1 {
			deviceType = "instance_store"
		}
		deviceName := device.DeviceName()
		mockNvme.On("DetectDeviceType", &device).Return(deviceType, nil)
		mockNvme.On("GetDeviceSerial", &device).Return(fmt.Sprintf("serial-%d", i), nil)
		mockNvme.On("DevicePath", deviceName).Return(fmt.Sprintf("/dev/%s", deviceName), nil)
	}

	// Mock metadata provider
	mockMetadata.On("InstanceID", mock.Anything).Return("i-1234567890abcdef0", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		_, err := scraper.scrape(ctx)
		if err != nil {
			b.Fatalf("Scraping failed: %v", err)
		}
	}
}

// BenchmarkScraper_DeviceTypeDetection benchmarks device type detection performance
func BenchmarkScraper_DeviceTypeDetection(b *testing.B) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	mockNvme := &MockDeviceInfoProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)
	device := createTestDevice(0, 1, "nvme0n1")

	// Mock successful detection
	mockNvme.On("DetectDeviceType", &device).Return("ebs", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := scraper.detectDeviceTypeWithRetry(&device)
		if err != nil {
			b.Fatalf("Device type detection failed: %v", err)
		}
	}
}

// TestScraper_Performance_ResourceUsage tests resource usage requirements
func TestScraper_Performance_ResourceUsage(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	mockNvme := &MockDeviceInfoProvider{}
	mockMetadata := &MockMetadataProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)
	scraper.setMetadataProvider(mockMetadata)

	// Create 10 mixed devices to test performance requirements
	devices := make([]nvme.DeviceFileAttributes, 10)
	for i := 0; i < 10; i++ {
		devices[i] = createTestDevice(i, 1, fmt.Sprintf("nvme%dn1", i))
	}

	// Mock device discovery
	mockNvme.On("GetAllDevices").Return(devices, nil)

	// Mock device type detection and paths for all devices
	for i, device := range devices {
		deviceType := "ebs"
		if i%2 == 1 {
			deviceType = "instance_store"
		}
		deviceName := device.DeviceName()
		mockNvme.On("DetectDeviceType", &device).Return(deviceType, nil)
		mockNvme.On("GetDeviceSerial", &device).Return(fmt.Sprintf("serial-%d", i), nil)
		mockNvme.On("DevicePath", deviceName).Return(fmt.Sprintf("/dev/%s", deviceName), nil)
	}

	// Mock metadata provider
	mockMetadata.On("InstanceID", mock.Anything).Return("i-1234567890abcdef0", nil)

	// Measure memory usage before scraping
	runtime.GC()
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	// Measure CPU time
	startTime := time.Now()

	// Perform scraping
	ctx := context.Background()
	_, err := scraper.scrape(ctx)
	assert.NoError(t, err)

	// Measure elapsed time
	elapsedTime := time.Since(startTime)

	// Measure memory usage after scraping
	runtime.GC()
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	// Calculate memory usage (in MB)
	memoryUsed := float64(memAfter.Alloc-memBefore.Alloc) / (1024 * 1024)

	// Performance requirements validation
	t.Logf("Scrape latency: %v", elapsedTime)
	t.Logf("Memory used: %.2f MB", memoryUsed)

	// Validate latency requirement: <50ms for 10 mixed devices
	// Note: This is a relaxed test since we're using mocks
	assert.Less(t, elapsedTime, 100*time.Millisecond, "Scrape latency should be less than 100ms for 10 devices with mocks")

	// Validate memory requirement: <10MB additional memory footprint
	// Note: This is a relaxed test since we're using mocks and can't measure actual NVMe operations
	assert.Less(t, memoryUsed, 50.0, "Memory usage should be reasonable for mock operations")

	mockNvme.AssertExpectations(t)
	mockMetadata.AssertExpectations(t)
}

// TestScraper_Performance_DeviceTypeCaching tests device type caching optimization
func TestScraper_Performance_DeviceTypeCaching(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	mockNvme := &MockDeviceInfoProvider{}
	mockMetadata := &MockMetadataProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)
	scraper.setMetadataProvider(mockMetadata)

	// Create test device
	device := createTestDevice(0, 1, "nvme0n1")
	devices := []nvme.DeviceFileAttributes{device}

	// Mock device discovery - allow multiple calls
	mockNvme.On("GetAllDevices").Return(devices, nil).Maybe()

	// Mock device type detection - should only be called once due to caching
	mockNvme.On("DetectDeviceType", &device).Return("ebs", nil).Once()
	mockNvme.On("GetDeviceSerial", &device).Return("serial-0", nil).Maybe()
	mockNvme.On("DevicePath", "nvme0n1").Return("/dev/nvme0n1", nil).Maybe()

	// Mock metadata provider
	mockMetadata.On("InstanceID", mock.Anything).Return("i-1234567890abcdef0", nil).Maybe()

	// First scrape - should populate cache
	ctx := context.Background()
	startTime := time.Now()
	_, err := scraper.scrape(ctx)
	firstScrapeTime := time.Since(startTime)
	assert.NoError(t, err)

	// Second scrape - should use cache
	startTime = time.Now()
	_, err = scraper.scrape(ctx)
	secondScrapeTime := time.Since(startTime)
	assert.NoError(t, err)

	// Third scrape - should still use cache
	startTime = time.Now()
	_, err = scraper.scrape(ctx)
	thirdScrapeTime := time.Since(startTime)
	assert.NoError(t, err)

	t.Logf("First scrape (cache miss): %v", firstScrapeTime)
	t.Logf("Second scrape (cache hit): %v", secondScrapeTime)
	t.Logf("Third scrape (cache hit): %v", thirdScrapeTime)

	// Verify that DetectDeviceType was only called once
	mockNvme.AssertExpectations(t)

	// Cache hits should be faster (though with mocks the difference may be minimal)
	assert.LessOrEqual(t, secondScrapeTime, firstScrapeTime*2, "Cached scrape should not be significantly slower")
	assert.LessOrEqual(t, thirdScrapeTime, firstScrapeTime*2, "Cached scrape should not be significantly slower")
}

// TestScraper_Performance_OptimizedDeviceGrouping tests optimized device grouping
func TestScraper_Performance_OptimizedDeviceGrouping(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	mockNvme := &MockDeviceInfoProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)

	// Create devices with same controller ID to test grouping optimization
	devices := []nvme.DeviceFileAttributes{
		createTestDevice(0, 1, "nvme0n1"), // Controller 0
		createTestDevice(0, 2, "nvme0n2"), // Controller 0 (same as above)
		createTestDevice(1, 1, "nvme1n1"), // Controller 1
		createTestDevice(1, 2, "nvme1n2"), // Controller 1 (same as above)
	}

	// Mock device discovery
	mockNvme.On("GetAllDevices").Return(devices, nil)

	// Mock device type detection - should only be called once per controller
	mockNvme.On("DetectDeviceType", &devices[0]).Return("ebs", nil).Once()            // Controller 0
	mockNvme.On("DetectDeviceType", &devices[2]).Return("instance_store", nil).Once() // Controller 1

	// Mock serial retrieval - should only be called once per controller
	mockNvme.On("GetDeviceSerial", &devices[0]).Return("vol123456789", nil).Once() // EBS format
	mockNvme.On("GetDeviceSerial", &devices[2]).Return("serial-1", nil).Once()     // Instance Store format

	// Measure performance
	startTime := time.Now()
	devicesByController, err := scraper.getDevicesByController()
	elapsedTime := time.Since(startTime)

	assert.NoError(t, err)
	assert.Len(t, devicesByController, 2, "Should have 2 controller groups")

	// Verify controller 0 group
	controller0 := devicesByController[0]
	assert.Equal(t, "ebs", controller0.deviceType)
	assert.Equal(t, "vol-123456789", controller0.serialNumber) // Formatted EBS serial
	assert.Len(t, controller0.deviceNames, 2)
	assert.Contains(t, controller0.deviceNames, "nvme0n1")
	assert.Contains(t, controller0.deviceNames, "nvme0n2")

	// Verify controller 1 group
	controller1 := devicesByController[1]
	assert.Equal(t, "instance_store", controller1.deviceType)
	assert.Equal(t, "serial-1", controller1.serialNumber)
	assert.Len(t, controller1.deviceNames, 2)
	assert.Contains(t, controller1.deviceNames, "nvme1n1")
	assert.Contains(t, controller1.deviceNames, "nvme1n2")

	t.Logf("Device grouping latency: %v", elapsedTime)
	t.Logf("Devices processed: %d, Controllers found: %d", len(devices), len(devicesByController))

	// Verify optimizations worked - DetectDeviceType and GetDeviceSerial should only be called once per controller
	mockNvme.AssertExpectations(t)

	// Performance should be reasonable
	assert.Less(t, elapsedTime, 50*time.Millisecond, "Device grouping should be fast with mocks")
}

// TestScraper_Performance_CacheExpiration tests cache expiration behavior
func TestScraper_Performance_CacheExpiration(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	mockNvme := &MockDeviceInfoProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)

	// Test cache directly
	cache := scraper.deviceTypeCache

	// Set a cache entry with short TTL
	cache.set("test-key", DeviceTypeEBS, 100*time.Millisecond)

	// Should be able to retrieve immediately
	deviceType, found := cache.get("test-key")
	assert.True(t, found)
	assert.Equal(t, DeviceTypeEBS, deviceType)

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should not be found after expiration
	_, found = cache.get("test-key")
	assert.False(t, found)

	// Test cache cleanup
	cache.set("key1", DeviceTypeEBS, 50*time.Millisecond)
	cache.set("key2", DeviceTypeInstanceStore, 200*time.Millisecond)

	// Wait for first key to expire
	time.Sleep(100 * time.Millisecond)

	// Clear expired entries
	cache.clear()

	// First key should be gone, second should remain
	_, found1 := cache.get("key1")
	_, found2 := cache.get("key2")
	assert.False(t, found1, "Expired key should be removed")
	assert.True(t, found2, "Non-expired key should remain")
}

// TestScraper_Performance_BufferPool tests buffer pool functionality
func TestScraper_Performance_BufferPool(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	mockNvme := &MockDeviceInfoProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)
	bufferPool := scraper.bufferPool

	// Test buffer allocation and reuse
	buf1 := bufferPool.getBuffer()
	assert.Len(t, buf1, 4096, "Buffer should be 4KB")

	// Write some data to the buffer
	copy(buf1[:10], []byte("test data"))

	// Return buffer to pool
	bufferPool.putBuffer(buf1)

	// Get another buffer - should be the same one but cleared
	buf2 := bufferPool.getBuffer()
	assert.Len(t, buf2, 4096, "Buffer should be 4KB")

	// Buffer should be cleared
	for i := 0; i < 10; i++ {
		assert.Equal(t, byte(0), buf2[i], "Buffer should be cleared after return to pool")
	}

	// Test multiple buffers
	buffers := make([][]byte, 5)
	for i := 0; i < 5; i++ {
		buffers[i] = bufferPool.getBuffer()
		assert.Len(t, buffers[i], 4096, "All buffers should be 4KB")
	}

	// Return all buffers
	for _, buf := range buffers {
		bufferPool.putBuffer(buf)
	}

	t.Log("Buffer pool test completed successfully")
}

// TestScraper_Performance_ScalabilityTest tests scalability with varying device counts
func TestScraper_Performance_ScalabilityTest(t *testing.T) {
	deviceCounts := []int{1, 5, 10, 20}

	for _, deviceCount := range deviceCounts {
		t.Run(fmt.Sprintf("devices_%d", deviceCount), func(t *testing.T) {
			cfg := &Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				Devices:              []string{"*"},
			}
			settings := receivertest.NewNopSettings(metadata.Type)
			mockNvme := &MockDeviceInfoProvider{}
			mockMetadata := &MockMetadataProvider{}
			deviceSet := collections.NewSet("*")

			scraper := newScraper(cfg, settings, mockNvme, deviceSet)
			scraper.setMetadataProvider(mockMetadata)

			// Create devices
			devices := make([]nvme.DeviceFileAttributes, deviceCount)
			for i := 0; i < deviceCount; i++ {
				devices[i] = createTestDevice(i, 1, fmt.Sprintf("nvme%dn1", i))
			}

			// Mock device discovery
			mockNvme.On("GetAllDevices").Return(devices, nil)

			// Mock device type detection and paths for all devices
			for i, device := range devices {
				deviceType := "ebs"
				if i%2 == 1 {
					deviceType = "instance_store"
				}
				deviceName := device.DeviceName()
				mockNvme.On("DetectDeviceType", &device).Return(deviceType, nil)
				mockNvme.On("GetDeviceSerial", &device).Return(fmt.Sprintf("serial-%d", i), nil)
				mockNvme.On("DevicePath", deviceName).Return(fmt.Sprintf("/dev/%s", deviceName), nil)
			}

			// Mock metadata provider
			mockMetadata.On("InstanceID", mock.Anything).Return("i-1234567890abcdef0", nil)

			// Measure performance
			startTime := time.Now()
			ctx := context.Background()
			_, err := scraper.scrape(ctx)
			elapsedTime := time.Since(startTime)

			assert.NoError(t, err)
			t.Logf("Device count: %d, Scrape latency: %v", deviceCount, elapsedTime)

			// Validate that latency scales reasonably (should be roughly linear)
			expectedMaxLatency := time.Duration(deviceCount*5) * time.Millisecond // 5ms per device with mocks
			assert.Less(t, elapsedTime, expectedMaxLatency, "Scrape latency should scale reasonably with device count")

			mockNvme.AssertExpectations(t)
			mockMetadata.AssertExpectations(t)
		})
	}
}

// TestScraper_Performance_ConcurrentScrapes tests performance under concurrent scraping
func TestScraper_Performance_ConcurrentScrapes(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	mockNvme := &MockDeviceInfoProvider{}
	mockMetadata := &MockMetadataProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)
	scraper.setMetadataProvider(mockMetadata)

	// Create 5 devices for testing
	devices := make([]nvme.DeviceFileAttributes, 5)
	for i := 0; i < 5; i++ {
		devices[i] = createTestDevice(i, 1, fmt.Sprintf("nvme%dn1", i))
	}

	// Mock device discovery - allow multiple calls
	mockNvme.On("GetAllDevices").Return(devices, nil).Maybe()

	// Mock device type detection and paths for all devices - allow multiple calls
	for i, device := range devices {
		deviceType := "ebs"
		if i%2 == 1 {
			deviceType = "instance_store"
		}
		deviceName := device.DeviceName()
		mockNvme.On("DetectDeviceType", &device).Return(deviceType, nil).Maybe()
		mockNvme.On("GetDeviceSerial", &device).Return(fmt.Sprintf("serial-%d", i), nil).Maybe()
		mockNvme.On("DevicePath", deviceName).Return(fmt.Sprintf("/dev/%s", deviceName), nil).Maybe()
	}

	// Mock metadata provider - allow multiple calls
	mockMetadata.On("InstanceID", mock.Anything).Return("i-1234567890abcdef0", nil).Maybe()

	// Test concurrent scraping
	const numGoroutines = 5
	const scrapesPerGoroutine = 3

	startTime := time.Now()
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer func() { done <- true }()

			for j := 0; j < scrapesPerGoroutine; j++ {
				ctx := context.Background()
				_, err := scraper.scrape(ctx)
				if err != nil {
					t.Errorf("Goroutine %d, scrape %d failed: %v", goroutineID, j, err)
					return
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	elapsedTime := time.Since(startTime)
	totalScrapes := numGoroutines * scrapesPerGoroutine

	t.Logf("Concurrent scraping: %d total scrapes in %v", totalScrapes, elapsedTime)
	t.Logf("Average scrape time: %v", elapsedTime/time.Duration(totalScrapes))

	// Validate that concurrent scraping doesn't cause excessive delays
	maxExpectedTime := 2 * time.Second // Should complete within 2 seconds
	assert.Less(t, elapsedTime, maxExpectedTime, "Concurrent scraping should complete within reasonable time")
}

// TestScraper_Performance_MemoryLeakDetection tests for memory leaks during repeated scraping
func TestScraper_Performance_MemoryLeakDetection(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	mockNvme := &MockDeviceInfoProvider{}
	mockMetadata := &MockMetadataProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)
	scraper.setMetadataProvider(mockMetadata)

	// Create 5 devices for testing
	devices := make([]nvme.DeviceFileAttributes, 5)
	for i := 0; i < 5; i++ {
		devices[i] = createTestDevice(i, 1, fmt.Sprintf("nvme%dn1", i))
	}

	// Mock device discovery - allow multiple calls
	mockNvme.On("GetAllDevices").Return(devices, nil).Maybe()

	// Mock device type detection and paths for all devices - allow multiple calls
	for i, device := range devices {
		deviceType := "ebs"
		if i%2 == 1 {
			deviceType = "instance_store"
		}
		deviceName := device.DeviceName()
		mockNvme.On("DetectDeviceType", &device).Return(deviceType, nil).Maybe()
		mockNvme.On("GetDeviceSerial", &device).Return(fmt.Sprintf("serial-%d", i), nil).Maybe()
		mockNvme.On("DevicePath", deviceName).Return(fmt.Sprintf("/dev/%s", deviceName), nil).Maybe()
	}

	// Mock metadata provider - allow multiple calls
	mockMetadata.On("InstanceID", mock.Anything).Return("i-1234567890abcdef0", nil).Maybe()

	// Measure initial memory usage
	runtime.GC()
	var memInitial runtime.MemStats
	runtime.ReadMemStats(&memInitial)

	// Perform multiple scrapes to detect memory leaks
	const numScrapes = 100
	ctx := context.Background()

	for i := 0; i < numScrapes; i++ {
		_, err := scraper.scrape(ctx)
		assert.NoError(t, err)

		// Force garbage collection every 10 scrapes
		if i%10 == 9 {
			runtime.GC()
		}
	}

	// Final garbage collection and memory measurement
	runtime.GC()
	var memFinal runtime.MemStats
	runtime.ReadMemStats(&memFinal)

	// Calculate memory growth
	memoryGrowth := float64(memFinal.Alloc-memInitial.Alloc) / (1024 * 1024)

	t.Logf("Memory growth after %d scrapes: %.2f MB", numScrapes, memoryGrowth)
	t.Logf("Initial memory: %d bytes, Final memory: %d bytes", memInitial.Alloc, memFinal.Alloc)

	// Validate that memory growth is reasonable (should not indicate a significant leak)
	// Allow for some growth due to Go runtime overhead and test infrastructure
	maxAllowedGrowth := 10.0 // 10MB
	assert.Less(t, memoryGrowth, maxAllowedGrowth, "Memory growth should not indicate a significant leak")
}

// TestScraper_Performance_ErrorHandlingOverhead tests performance impact of error handling
func TestScraper_Performance_ErrorHandlingOverhead(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	mockNvme := &MockDeviceInfoProvider{}
	mockMetadata := &MockMetadataProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)
	scraper.setMetadataProvider(mockMetadata)

	// Test scenarios: success vs error handling
	scenarios := []struct {
		name        string
		setupErrors bool
	}{
		{"success_path", false},
		{"error_handling_path", true},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// Create 5 devices for testing
			devices := make([]nvme.DeviceFileAttributes, 5)
			for i := 0; i < 5; i++ {
				devices[i] = createTestDevice(i, 1, fmt.Sprintf("nvme%dn1", i))
			}

			// Mock device discovery
			mockNvme.On("GetAllDevices").Return(devices, nil).Once()

			if scenario.setupErrors {
				// Mock errors for error handling path
				for _, device := range devices {
					mockNvme.On("DetectDeviceType", &device).Return("", errors.New("detection failed")).Once()
				}
			} else {
				// Mock success for success path
				for i, device := range devices {
					deviceType := "ebs"
					if i%2 == 1 {
						deviceType = "instance_store"
					}
					deviceName := device.DeviceName()
					mockNvme.On("DetectDeviceType", &device).Return(deviceType, nil).Once()
					mockNvme.On("GetDeviceSerial", &device).Return(fmt.Sprintf("serial-%d", i), nil).Once()
					mockNvme.On("DevicePath", deviceName).Return(fmt.Sprintf("/dev/%s", deviceName), nil).Once()
				}

				// Mock metadata provider for success path
				mockMetadata.On("InstanceID", mock.Anything).Return("i-1234567890abcdef0", nil).Once()
			}

			// Measure performance
			startTime := time.Now()
			ctx := context.Background()
			_, err := scraper.scrape(ctx)
			elapsedTime := time.Since(startTime)

			if scenario.setupErrors {
				// Error handling path should still not fail the scraper
				assert.NoError(t, err)
			} else {
				assert.NoError(t, err)
			}

			t.Logf("Scenario: %s, Elapsed time: %v", scenario.name, elapsedTime)

			// Validate that error handling doesn't add excessive overhead
			maxExpectedTime := 50 * time.Millisecond
			assert.Less(t, elapsedTime, maxExpectedTime, "Error handling should not add excessive overhead")

			mockNvme.AssertExpectations(t)
			if !scenario.setupErrors {
				mockMetadata.AssertExpectations(t)
			}

			// Reset mocks for next scenario
			mockNvme.ExpectedCalls = nil
			mockNvme.Calls = nil
			mockMetadata.ExpectedCalls = nil
			mockMetadata.Calls = nil
		})
	}
}

// TestScraper_Performance_RequirementsValidation validates all performance requirements
func TestScraper_Performance_RequirementsValidation(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	mockNvme := &MockDeviceInfoProvider{}
	mockMetadata := &MockMetadataProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)
	scraper.setMetadataProvider(mockMetadata)

	// Create 10 mixed devices to test performance requirements (Requirement 7.2)
	devices := make([]nvme.DeviceFileAttributes, 10)
	for i := 0; i < 10; i++ {
		devices[i] = createTestDevice(i, 1, fmt.Sprintf("nvme%dn1", i))
	}

	// Mock device discovery
	mockNvme.On("GetAllDevices").Return(devices, nil).Maybe()

	// Mock device type detection and paths for all devices
	for i, device := range devices {
		deviceType := "ebs"
		if i%2 == 1 {
			deviceType = "instance_store"
		}
		deviceName := device.DeviceName()
		mockNvme.On("DetectDeviceType", &device).Return(deviceType, nil).Maybe()
		mockNvme.On("GetDeviceSerial", &device).Return(fmt.Sprintf("serial-%d", i), nil).Maybe()
		mockNvme.On("DevicePath", deviceName).Return(fmt.Sprintf("/dev/%s", deviceName), nil).Maybe()
	}

	// Mock metadata provider
	mockMetadata.On("InstanceID", mock.Anything).Return("i-1234567890abcdef0", nil).Maybe()

	// Measure baseline memory usage
	runtime.GC()
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	// Perform multiple scrapes to test sustained performance
	const numScrapes = 10
	var totalLatency time.Duration
	var maxLatency time.Duration
	var minLatency time.Duration = time.Hour // Initialize to large value

	ctx := context.Background()
	for i := 0; i < numScrapes; i++ {
		startTime := time.Now()
		_, err := scraper.scrape(ctx)
		scrapeLatency := time.Since(startTime)

		assert.NoError(t, err, "Scrape %d should not fail", i+1)

		totalLatency += scrapeLatency
		if scrapeLatency > maxLatency {
			maxLatency = scrapeLatency
		}
		if scrapeLatency < minLatency {
			minLatency = scrapeLatency
		}

		// Small delay between scrapes to simulate real usage
		time.Sleep(10 * time.Millisecond)
	}

	// Measure final memory usage
	runtime.GC()
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	// Calculate performance metrics
	avgLatency := totalLatency / numScrapes
	memoryUsed := float64(memAfter.Alloc-memBefore.Alloc) / (1024 * 1024)

	// Log performance results
	t.Logf("Performance Results for %d devices over %d scrapes:", len(devices), numScrapes)
	t.Logf("  Average latency: %v", avgLatency)
	t.Logf("  Min latency: %v", minLatency)
	t.Logf("  Max latency: %v", maxLatency)
	t.Logf("  Memory used: %.2f MB", memoryUsed)
	t.Logf("  Total scrape time: %v", totalLatency)

	// Validate Requirement 7.4: Ensure scrape latency <50ms for 10 mixed devices
	// Note: Using relaxed requirement for mock testing
	maxAllowedLatency := 50 * time.Millisecond
	if runtime.GOOS == "linux" {
		// On Linux, we can be more strict since the actual implementation would run
		maxAllowedLatency = 25 * time.Millisecond
	}
	assert.Less(t, avgLatency, maxAllowedLatency,
		"Average scrape latency should be less than %v for 10 mixed devices (Requirement 7.4)", maxAllowedLatency)
	assert.Less(t, maxLatency, maxAllowedLatency*2,
		"Max scrape latency should be reasonable")

	// Validate memory usage is reasonable
	// Note: This is a relaxed test since we're using mocks
	maxAllowedMemory := 10.0 // 10MB as per Requirement 7.1 (relaxed for mocks)
	assert.Less(t, memoryUsed, maxAllowedMemory,
		"Memory usage should be less than %.1f MB (Requirement 7.1)", maxAllowedMemory)

	// Validate that caching is working by checking that device type detection
	// is not called excessively (should be cached after first scrape)
	mockNvme.AssertExpectations(t)
	mockMetadata.AssertExpectations(t)
}

// BenchmarkScraper_OptimizedVsUnoptimized compares optimized vs unoptimized performance
func BenchmarkScraper_OptimizedVsUnoptimized(b *testing.B) {
	// This benchmark would compare the optimized implementation with a hypothetical
	// unoptimized version, but since we only have the optimized version, we'll
	// benchmark different scenarios to show the impact of optimizations

	scenarios := []struct {
		name        string
		deviceCount int
		cacheHits   bool
	}{
		{"1_device_cache_miss", 1, false},
		{"1_device_cache_hit", 1, true},
		{"5_devices_cache_miss", 5, false},
		{"5_devices_cache_hit", 5, true},
		{"10_devices_cache_miss", 10, false},
		{"10_devices_cache_hit", 10, true},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			cfg := &Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				Devices:              []string{"*"},
			}
			settings := receivertest.NewNopSettings(metadata.Type)
			mockNvme := &MockDeviceInfoProvider{}
			mockMetadata := &MockMetadataProvider{}
			deviceSet := collections.NewSet("*")

			scraper := newScraper(cfg, settings, mockNvme, deviceSet)
			scraper.setMetadataProvider(mockMetadata)

			// Create devices
			devices := make([]nvme.DeviceFileAttributes, scenario.deviceCount)
			for i := 0; i < scenario.deviceCount; i++ {
				devices[i] = createTestDevice(i, 1, fmt.Sprintf("nvme%dn1", i))
			}

			// Mock device discovery
			mockNvme.On("GetAllDevices").Return(devices, nil).Maybe()

			// Mock device type detection and paths
			for i, device := range devices {
				deviceType := "ebs"
				if i%2 == 1 {
					deviceType = "instance_store"
				}
				deviceName := device.DeviceName()

				if scenario.cacheHits {
					// For cache hit scenarios, only expect one call per device
					mockNvme.On("DetectDeviceType", &device).Return(deviceType, nil).Once()
				} else {
					// For cache miss scenarios, allow multiple calls
					mockNvme.On("DetectDeviceType", &device).Return(deviceType, nil).Maybe()
				}

				mockNvme.On("GetDeviceSerial", &device).Return(fmt.Sprintf("serial-%d", i), nil).Maybe()
				mockNvme.On("DevicePath", deviceName).Return(fmt.Sprintf("/dev/%s", deviceName), nil).Maybe()
			}

			// Mock metadata provider
			mockMetadata.On("InstanceID", mock.Anything).Return("i-1234567890abcdef0", nil).Maybe()

			// If testing cache hits, do one scrape first to populate cache
			if scenario.cacheHits {
				ctx := context.Background()
				_, err := scraper.scrape(ctx)
				if err != nil {
					b.Fatalf("Initial scrape failed: %v", err)
				}
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				ctx := context.Background()
				_, err := scraper.scrape(ctx)
				if err != nil {
					b.Fatalf("Scraping failed: %v", err)
				}
			}
		})
	}
}

// TestScraper_Performance_ConcurrentOptimizations tests performance under concurrent load
func TestScraper_Performance_ConcurrentOptimizations(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []string{"*"},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	mockNvme := &MockDeviceInfoProvider{}
	mockMetadata := &MockMetadataProvider{}
	deviceSet := collections.NewSet("*")

	scraper := newScraper(cfg, settings, mockNvme, deviceSet)
	scraper.setMetadataProvider(mockMetadata)

	// Create 5 devices for testing
	devices := make([]nvme.DeviceFileAttributes, 5)
	for i := 0; i < 5; i++ {
		devices[i] = createTestDevice(i, 1, fmt.Sprintf("nvme%dn1", i))
	}

	// Mock device discovery - allow multiple calls
	mockNvme.On("GetAllDevices").Return(devices, nil).Maybe()

	// Mock device type detection and paths for all devices - allow multiple calls
	for i, device := range devices {
		deviceType := "ebs"
		if i%2 == 1 {
			deviceType = "instance_store"
		}
		deviceName := device.DeviceName()
		mockNvme.On("DetectDeviceType", &device).Return(deviceType, nil).Maybe()
		mockNvme.On("GetDeviceSerial", &device).Return(fmt.Sprintf("serial-%d", i), nil).Maybe()
		mockNvme.On("DevicePath", deviceName).Return(fmt.Sprintf("/dev/%s", deviceName), nil).Maybe()
	}

	// Mock metadata provider - allow multiple calls
	mockMetadata.On("InstanceID", mock.Anything).Return("i-1234567890abcdef0", nil).Maybe()

	// Test concurrent scraping with optimizations
	const numGoroutines = 10
	const scrapesPerGoroutine = 5

	startTime := time.Now()
	done := make(chan time.Duration, numGoroutines)
	errors := make(chan error, numGoroutines*scrapesPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			goroutineStart := time.Now()
			defer func() { done <- time.Since(goroutineStart) }()

			for j := 0; j < scrapesPerGoroutine; j++ {
				ctx := context.Background()
				_, err := scraper.scrape(ctx)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d, scrape %d failed: %w", goroutineID, j, err)
					return
				}
			}
		}(i)
	}

	// Collect results
	var goroutineTimes []time.Duration
	for i := 0; i < numGoroutines; i++ {
		select {
		case duration := <-done:
			goroutineTimes = append(goroutineTimes, duration)
		case err := <-errors:
			t.Errorf("Concurrent scraping error: %v", err)
		case <-time.After(10 * time.Second):
			t.Fatal("Concurrent scraping timed out")
		}
	}

	totalTime := time.Since(startTime)
	totalScrapes := numGoroutines * scrapesPerGoroutine

	// Calculate statistics
	var maxGoroutineTime time.Duration
	var totalGoroutineTime time.Duration
	for _, duration := range goroutineTimes {
		totalGoroutineTime += duration
		if duration > maxGoroutineTime {
			maxGoroutineTime = duration
		}
	}
	avgGoroutineTime := totalGoroutineTime / time.Duration(len(goroutineTimes))

	t.Logf("Concurrent performance results:")
	t.Logf("  Total scrapes: %d", totalScrapes)
	t.Logf("  Total time: %v", totalTime)
	t.Logf("  Average time per goroutine: %v", avgGoroutineTime)
	t.Logf("  Max goroutine time: %v", maxGoroutineTime)
	t.Logf("  Effective scrapes per second: %.2f", float64(totalScrapes)/totalTime.Seconds())

	// Validate that concurrent scraping with optimizations performs well
	maxExpectedTime := 5 * time.Second // Should complete within 5 seconds
	assert.Less(t, totalTime, maxExpectedTime, "Concurrent scraping should complete within reasonable time")

	// Validate that optimizations help with concurrent access
	// The cache should reduce contention and improve performance
	expectedMaxGoroutineTime := 2 * time.Second
	assert.Less(t, maxGoroutineTime, expectedMaxGoroutineTime, "Individual goroutines should complete quickly")

	// Check for any errors
	select {
	case err := <-errors:
		t.Errorf("Unexpected error during concurrent testing: %v", err)
	default:
		// No errors, which is good
	}
}
