// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatchlogs

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/plugins/outputs/cloudwatchlogs/internal/pusher"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
)

// TestTruncationIntegration tests the complete truncation flow from input to output
func TestTruncationIntegration(t *testing.T) {
	// This test verifies that the truncation fix works end-to-end
	
	t.Run("CompleteFlow256KB", func(t *testing.T) {
		// Create a CloudWatch Logs output plugin instance
		logger := testutil.NewNopLogger()
		
		// Test with the corrected 256KB limits
		testCases := []struct {
			name           string
			messageSize    int
			shouldTruncate bool
			description    string
		}{
			{
				name:           "SmallMessage",
				messageSize:    1000,
				shouldTruncate: false,
				description:    "Small messages should pass through unchanged",
			},
			{
				name:           "MediumMessage",
				messageSize:    100000, // 100KB
				shouldTruncate: false,
				description:    "Medium messages should pass through unchanged",
			},
			{
				name:           "LargeButValidMessage",
				messageSize:    250000, // ~244KB
				shouldTruncate: false,
				description:    "Large but valid messages should pass through unchanged",
			},
			{
				name:           "MessageAtLimit",
				messageSize:    262118, // 256KB - 26 bytes (exact limit)
				shouldTruncate: false,
				description:    "Messages at the exact limit should not be truncated",
			},
			{
				name:           "MessageOverLimit",
				messageSize:    300000, // ~293KB
				shouldTruncate: true,
				description:    "Messages over the limit should be truncated",
			},
			{
				name:           "VeryLargeMessage",
				messageSize:    1000000, // ~977KB
				shouldTruncate: true,
				description:    "Very large messages should be truncated",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Create a test message of the specified size
				message := strings.Repeat("A", tc.messageSize)
				
				// Create a log event
				event := &testLogEvent{
					message:   message,
					timestamp: time.Now(),
				}

				// Create a converter (this is where truncation happens)
				target := pusher.Target{Group: "test-group", Stream: "test-stream"}
				conv := newConverter(logger, target)
				
				// Convert the event (this applies truncation if needed)
				logEvent := conv.convert(event)
				
				if tc.shouldTruncate {
					// Verify truncation occurred
					assert.Less(t, len(logEvent.message), tc.messageSize, 
						"Truncated message should be shorter than original")
					assert.Equal(t, 262118, len(logEvent.message), // 256KB - 26 bytes
						"Truncated message should be exactly at the limit")
					assert.True(t, strings.HasSuffix(logEvent.message, "[Truncated...]"), 
						"Truncated message should end with truncation suffix")
				} else {
					// Verify no truncation occurred
					assert.Equal(t, tc.messageSize, len(logEvent.message), 
						"Non-truncated message should maintain original size")
					assert.Equal(t, message, logEvent.message, 
						"Non-truncated message should be identical to original")
					assert.False(t, strings.HasSuffix(logEvent.message, "[Truncated...]"), 
						"Non-truncated message should not have truncation suffix")
				}
			})
		}
	})
}

// TestTruncationImprovement tests the improvement from the header bytes fix
func TestTruncationImprovement(t *testing.T) {
	t.Run("HeaderBytesImprovement", func(t *testing.T) {
		logger := testutil.NewNopLogger()
		target := pusher.Target{Group: "test-group", Stream: "test-stream"}
		conv := newConverter(logger, target)

		// Test a message size that would have been truncated with the old header calculation
		// but should not be truncated with the new calculation
		
		// Old limit: 256KB - 200 bytes = 261,944 bytes
		// New limit: 256KB - 26 bytes = 262,118 bytes
		// Improvement: 174 bytes
		
		improvementZoneSize := 262000 // Between old and new limits
		message := strings.Repeat("B", improvementZoneSize)
		
		event := &testLogEvent{
			message:   message,
			timestamp: time.Now(),
		}

		logEvent := conv.convert(event)
		
		// With the fix, this message should NOT be truncated
		assert.Equal(t, improvementZoneSize, len(logEvent.message), 
			"Message in improvement zone should not be truncated with the fix")
		assert.Equal(t, message, logEvent.message, 
			"Message should be identical to original")
		assert.False(t, strings.HasSuffix(logEvent.message, "[Truncated...]"), 
			"Message should not have truncation suffix")
	})
}

// TestBatchProcessing tests that batching works correctly with the truncation fix
func TestBatchProcessing(t *testing.T) {
	t.Run("BatchWithMixedMessageSizes", func(t *testing.T) {
		logger := testutil.NewNopLogger()
		target := pusher.Target{Group: "test-group", Stream: "test-stream"}
		
		// Create a batch with mixed message sizes
		messages := []struct {
			content        string
			shouldTruncate bool
		}{
			{"Small message", false},
			{strings.Repeat("M", 100000), false},  // 100KB - should not truncate
			{strings.Repeat("L", 300000), true},   // 300KB - should truncate
			{"Another small message", false},
			{strings.Repeat("X", 500000), true},   // 500KB - should truncate
		}

		batch := newLogEventBatch(target, nil)
		conv := newConverter(logger, target)

		for i, msg := range messages {
			event := &testLogEvent{
				message:   msg.content,
				timestamp: time.Now().Add(time.Duration(i) * time.Second),
			}
			
			logEvent := conv.convert(event)
			
			// Verify truncation behavior
			if msg.shouldTruncate {
				assert.Less(t, len(logEvent.message), len(msg.content), 
					"Large message should be truncated")
				assert.True(t, strings.HasSuffix(logEvent.message, "[Truncated...]"), 
					"Truncated message should have suffix")
			} else {
				assert.Equal(t, msg.content, logEvent.message, 
					"Small/medium message should not be truncated")
			}
			
			// Add to batch
			batch.append(logEvent)
		}

		// Verify batch can be built successfully
		input := batch.build()
		require.NotNil(t, input, "Batch should build successfully")
		assert.Equal(t, len(messages), len(input.LogEvents), 
			"Batch should contain all events")
	})
}

// TestMemoryEfficiency tests that the 256KB limit is memory efficient
func TestMemoryEfficiency(t *testing.T) {
	t.Run("MemoryUsageWithTruncation", func(t *testing.T) {
		logger := testutil.NewNopLogger()
		target := pusher.Target{Group: "test-group", Stream: "test-stream"}
		conv := newConverter(logger, target)

		// Create many large messages that will be truncated
		numMessages := 100
		largeMessageSize := 1000000 // 1MB each
		
		var totalProcessedSize int
		
		for i := 0; i < numMessages; i++ {
			message := strings.Repeat("D", largeMessageSize)
			event := &testLogEvent{
				message:   message,
				timestamp: time.Now(),
			}
			
			logEvent := conv.convert(event)
			totalProcessedSize += len(logEvent.message)
		}

		// With truncation, total processed size should be much smaller
		maxExpectedSize := numMessages * 262118 // 100 * (256KB - 26 bytes)
		assert.Equal(t, maxExpectedSize, totalProcessedSize, 
			"Total processed size should be limited by truncation")
		
		// Should be significantly less than if we processed full messages
		fullSize := numMessages * largeMessageSize
		assert.Less(t, totalProcessedSize, fullSize/3, 
			"Truncated size should be much smaller than full size")
	})
}

// TestBackwardCompatibility tests that existing functionality still works
func TestBackwardCompatibility(t *testing.T) {
	t.Run("ExistingConfigurationStillWorks", func(t *testing.T) {
		logger := testutil.NewNopLogger()
		target := pusher.Target{Group: "existing-group", Stream: "existing-stream"}
		conv := newConverter(logger, target)

		// Test typical log messages that would have worked before
		testMessages := []string{
			"INFO: Application started successfully",
			"ERROR: Database connection failed",
			"DEBUG: Processing user request ID 12345",
			strings.Repeat("Long log entry with repeated content ", 100),
		}

		for _, msg := range testMessages {
			event := &testLogEvent{
				message:   msg,
				timestamp: time.Now(),
			}
			
			logEvent := conv.convert(event)
			
			// These messages should all pass through unchanged
			assert.Equal(t, msg, logEvent.message, 
				"Existing message format should work unchanged")
		}
	})
}

// testLogEvent implements the logs.LogEvent interface for testing
type testLogEvent struct {
	message   string
	timestamp time.Time
	done      func()
}

func (e *testLogEvent) Message() string {
	return e.message
}

func (e *testLogEvent) Time() time.Time {
	return e.timestamp
}

func (e *testLogEvent) Done() {
	if e.done != nil {
		e.done()
	}
}

// Ensure testLogEvent implements logs.LogEvent
var _ logs.LogEvent = (*testLogEvent)(nil)
