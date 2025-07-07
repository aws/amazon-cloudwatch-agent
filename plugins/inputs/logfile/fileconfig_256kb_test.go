// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDefaultMaxEventSize verifies that the default max event size is set to 256KB
func TestDefaultMaxEventSize(t *testing.T) {
	t.Run("DefaultConstant", func(t *testing.T) {
		expected := 1024 * 256 // 256KB
		assert.Equal(t, expected, defaultMaxEventSize, "Default max event size should be 256KB")
		assert.Equal(t, 262144, defaultMaxEventSize, "Default max event size should be exactly 262144 bytes")
	})
}

// TestFileConfigInitialization tests that FileConfig initializes with correct default values
func TestFileConfigInitialization(t *testing.T) {
	t.Run("DefaultMaxEventSizeInitialization", func(t *testing.T) {
		config := &FileConfig{
			FilePath:      "/tmp/test.log",
			LogGroupName:  "test-group",
			LogStreamName: "test-stream",
			// MaxEventSize is not set, should use default
		}

		err := config.init()
		assert.NoError(t, err, "Config initialization should succeed")
		assert.Equal(t, defaultMaxEventSize, config.MaxEventSize, 
			"MaxEventSize should be set to default value when not specified")
		assert.Equal(t, 262144, config.MaxEventSize, 
			"MaxEventSize should be exactly 262144 bytes (256KB)")
	})

	t.Run("CustomMaxEventSizePreserved", func(t *testing.T) {
		customSize := 1024 * 128 // 128KB
		config := &FileConfig{
			FilePath:      "/tmp/test.log",
			LogGroupName:  "test-group",
			LogStreamName: "test-stream",
			MaxEventSize:  customSize,
		}

		err := config.init()
		assert.NoError(t, err, "Config initialization should succeed")
		assert.Equal(t, customSize, config.MaxEventSize, 
			"Custom MaxEventSize should be preserved")
		assert.NotEqual(t, defaultMaxEventSize, config.MaxEventSize, 
			"Custom MaxEventSize should not be overridden by default")
	})

	t.Run("ZeroMaxEventSizeUsesDefault", func(t *testing.T) {
		config := &FileConfig{
			FilePath:      "/tmp/test.log",
			LogGroupName:  "test-group",
			LogStreamName: "test-stream",
			MaxEventSize:  0, // Explicitly set to 0
		}

		err := config.init()
		assert.NoError(t, err, "Config initialization should succeed")
		assert.Equal(t, defaultMaxEventSize, config.MaxEventSize, 
			"Zero MaxEventSize should be replaced with default value")
	})
}

// TestMaxEventSizeValidation tests validation of max event size values
func TestMaxEventSizeValidation(t *testing.T) {
	testCases := []struct {
		name         string
		maxEventSize int
		expectError  bool
		description  string
	}{
		{
			name:         "ValidSmallSize",
			maxEventSize: 1024, // 1KB
			expectError:  false,
			description:  "Small valid size should be accepted",
		},
		{
			name:         "ValidDefaultSize",
			maxEventSize: defaultMaxEventSize,
			expectError:  false,
			description:  "Default size should be valid",
		},
		{
			name:         "ValidLargeSize",
			maxEventSize: 1024 * 512, // 512KB
			expectError:  false,
			description:  "Large size should be accepted",
		},
		{
			name:         "ZeroSize",
			maxEventSize: 0,
			expectError:  false, // Should use default, not error
			description:  "Zero size should use default",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &FileConfig{
				FilePath:      "/tmp/test.log",
				LogGroupName:  "test-group",
				LogStreamName: "test-stream",
				MaxEventSize:  tc.maxEventSize,
			}

			err := config.init()
			
			if tc.expectError {
				assert.Error(t, err, tc.description)
			} else {
				assert.NoError(t, err, tc.description)
				if tc.maxEventSize == 0 {
					assert.Equal(t, defaultMaxEventSize, config.MaxEventSize, 
						"Zero size should be replaced with default")
				} else {
					assert.Equal(t, tc.maxEventSize, config.MaxEventSize, 
						"Valid size should be preserved")
				}
			}
		})
	}
}

// TestTruncateSuffixDefault tests that the default truncate suffix is set correctly
func TestTruncateSuffixDefault(t *testing.T) {
	t.Run("DefaultTruncateSuffix", func(t *testing.T) {
		expected := "[Truncated...]"
		assert.Equal(t, expected, defaultTruncateSuffix, 
			"Default truncate suffix should be '[Truncated...]'")
	})

	t.Run("TruncateSuffixInitialization", func(t *testing.T) {
		config := &FileConfig{
			FilePath:      "/tmp/test.log",
			LogGroupName:  "test-group",
			LogStreamName: "test-stream",
			// TruncateSuffix not set, should use default
		}

		err := config.init()
		assert.NoError(t, err, "Config initialization should succeed")
		assert.Equal(t, defaultTruncateSuffix, config.TruncateSuffix, 
			"TruncateSuffix should be set to default value when not specified")
	})

	t.Run("CustomTruncateSuffixPreserved", func(t *testing.T) {
		customSuffix := "[TRUNCATED]"
		config := &FileConfig{
			FilePath:       "/tmp/test.log",
			LogGroupName:   "test-group",
			LogStreamName:  "test-stream",
			TruncateSuffix: customSuffix,
		}

		err := config.init()
		assert.NoError(t, err, "Config initialization should succeed")
		assert.Equal(t, customSuffix, config.TruncateSuffix, 
			"Custom TruncateSuffix should be preserved")
	})
}

// TestConfigurationCompatibility tests that the configuration remains compatible
func TestConfigurationCompatibility(t *testing.T) {
	t.Run("BackwardCompatibility", func(t *testing.T) {
		// Test that existing configurations without MaxEventSize still work
		config := &FileConfig{
			FilePath:      "/var/log/application.log",
			LogGroupName:  "application-logs",
			LogStreamName: "instance-1",
			FromBeginning: true,
		}

		err := config.init()
		assert.NoError(t, err, "Existing config format should still work")
		assert.Equal(t, defaultMaxEventSize, config.MaxEventSize, 
			"Should use default MaxEventSize for backward compatibility")
		assert.Equal(t, defaultTruncateSuffix, config.TruncateSuffix, 
			"Should use default TruncateSuffix for backward compatibility")
	})

	t.Run("AllFieldsInitialized", func(t *testing.T) {
		config := &FileConfig{
			FilePath:         "/var/log/app.log",
			LogGroupName:     "app-logs",
			LogStreamName:    "stream-1",
			MaxEventSize:     1024 * 128, // 128KB
			TruncateSuffix:   "[CUT]",
			FromBeginning:    true,
			TimestampRegex:   `\d{4}-\d{2}-\d{2}`,
			TimestampLayout:  []string{"2006-01-02"},
			Timezone:         "UTC",
			RetentionInDays:  7,
		}

		err := config.init()
		assert.NoError(t, err, "Full config should initialize successfully")
		
		// Verify all fields are properly set
		assert.Equal(t, 1024*128, config.MaxEventSize, "Custom MaxEventSize should be preserved")
		assert.Equal(t, "[CUT]", config.TruncateSuffix, "Custom TruncateSuffix should be preserved")
		assert.Equal(t, 7, config.RetentionInDays, "RetentionInDays should be preserved")
		assert.NotNil(t, config.TimestampRegexP, "TimestampRegex should be compiled")
	})
}

// TestSizeComparison tests the size differences between old and new limits
func TestSizeComparison(t *testing.T) {
	t.Run("SizeLimitComparison", func(t *testing.T) {
		// Current limit (256KB)
		currentLimit := defaultMaxEventSize
		assert.Equal(t, 262144, currentLimit, "Current limit should be 256KB")
		
		// If we were using 1MB (what was temporarily implemented)
		oneMBLimit := 1024 * 1024
		assert.Equal(t, 1048576, oneMBLimit, "1MB limit would be 1048576 bytes")
		
		// Verify we're using the smaller, more conservative limit
		assert.Less(t, currentLimit, oneMBLimit, "Current limit should be smaller than 1MB")
		
		// The difference should be significant
		difference := oneMBLimit - currentLimit
		assert.Equal(t, 786432, difference, "Difference should be 768KB")
	})
}

// TestMemoryEfficiency tests that the 256KB limit is memory efficient
func TestMemoryEfficiency(t *testing.T) {
	t.Run("ReasonableMemoryUsage", func(t *testing.T) {
		// With 256KB limit, memory usage should be reasonable
		maxEventSize := defaultMaxEventSize
		
		// Simulate processing multiple events
		numEvents := 100
		totalMemory := maxEventSize * numEvents
		
		// Total memory for 100 max-size events should be reasonable (25MB)
		expectedMemory := 26214400 // 100 * 262144
		assert.Equal(t, expectedMemory, totalMemory, 
			"Memory usage for 100 max-size events should be predictable")
		
		// Should be less than 30MB for 100 events
		assert.Less(t, totalMemory, 30*1024*1024, 
			"Memory usage should be reasonable for batch processing")
	})
}
