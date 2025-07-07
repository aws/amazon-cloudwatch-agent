// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
)

// TestTruncationFixConstants verifies that the constants are set correctly for 256KB limits
func TestTruncationFixConstants(t *testing.T) {
	t.Run("PerEventHeaderBytes", func(t *testing.T) {
		// Verify that per-event header bytes is set to the correct AWS API specification value
		assert.Equal(t, 26, perEventHeaderBytes, "Per-event header bytes should be 26 as per AWS PutLogEvents API specification")
	})

	t.Run("MessageSizeLimit", func(t *testing.T) {
		// Verify that message size limit is calculated correctly for 256KB
		expectedLimit := 256*1024 - perEventHeaderBytes // 256KB - 26 bytes
		assert.Equal(t, expectedLimit, msgSizeLimit, "Message size limit should be 256KB minus per-event header bytes")
		assert.Equal(t, 262118, msgSizeLimit, "Message size limit should be exactly 262118 bytes (256KB - 26 bytes)")
	})

	t.Run("TruncatedSuffix", func(t *testing.T) {
		assert.Equal(t, "[Truncated...]", truncatedSuffix, "Truncated suffix should be '[Truncated...]'")
	})
}

// TestMessageTruncationBehavior tests the message truncation logic with the corrected header bytes
func TestMessageTruncationBehavior(t *testing.T) {
	logger := testutil.NewNopLogger()
	target := Target{Group: "testGroup", Stream: "testStream"}
	conv := newConverter(logger, target)

	testCases := []struct {
		name           string
		messageSize    int
		shouldTruncate bool
		description    string
	}{
		{
			name:           "SmallMessage",
			messageSize:    100,
			shouldTruncate: false,
			description:    "Small messages should not be truncated",
		},
		{
			name:           "ExactLimit",
			messageSize:    msgSizeLimit,
			shouldTruncate: false,
			description:    "Messages exactly at the limit should not be truncated",
		},
		{
			name:           "OneBytePastLimit",
			messageSize:    msgSizeLimit + 1,
			shouldTruncate: true,
			description:    "Messages one byte past the limit should be truncated",
		},
		{
			name:           "LargeMessage",
			messageSize:    msgSizeLimit + 1000,
			shouldTruncate: true,
			description:    "Large messages should be truncated",
		},
		{
			name:           "VeryLargeMessage",
			messageSize:    1024 * 1024, // 1MB
			shouldTruncate: true,
			description:    "Very large messages should be truncated",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a message of the specified size
			message := strings.Repeat("A", tc.messageSize)
			event := newStubLogEvent(message, time.Now())

			// Convert the event
			logEvent := conv.convert(event)

			if tc.shouldTruncate {
				// Verify truncation occurred
				assert.Equal(t, msgSizeLimit, len(logEvent.message), 
					"Truncated message should be exactly msgSizeLimit bytes")
				assert.True(t, strings.HasSuffix(logEvent.message, truncatedSuffix), 
					"Truncated message should end with truncated suffix")
				
				// Verify the content before the suffix is correct
				expectedContentLength := msgSizeLimit - len(truncatedSuffix)
				expectedContent := strings.Repeat("A", expectedContentLength)
				actualContent := logEvent.message[:expectedContentLength]
				assert.Equal(t, expectedContent, actualContent, 
					"Content before truncated suffix should match original message")
			} else {
				// Verify no truncation occurred
				assert.Equal(t, tc.messageSize, len(logEvent.message), 
					"Non-truncated message should maintain original size")
				assert.Equal(t, message, logEvent.message, 
					"Non-truncated message should match original message")
				assert.False(t, strings.HasSuffix(logEvent.message, truncatedSuffix), 
					"Non-truncated message should not have truncated suffix")
			}
		})
	}
}

// TestTruncationImprovement verifies the improvement from the fix
func TestTruncationImprovement(t *testing.T) {
	t.Run("AdditionalBytesAvailable", func(t *testing.T) {
		// With the old incorrect header size of 200 bytes
		oldMsgSizeLimit := 256*1024 - 200 // 261944 bytes
		
		// With the new correct header size of 26 bytes
		newMsgSizeLimit := 256*1024 - 26 // 262118 bytes
		
		improvement := newMsgSizeLimit - oldMsgSizeLimit
		assert.Equal(t, 174, improvement, "The fix should provide 174 additional bytes for log content")
		assert.Equal(t, newMsgSizeLimit, msgSizeLimit, "Current msgSizeLimit should use the corrected header size")
	})
}

// TestLogEventBytesCalculation tests that log event bytes are calculated correctly
func TestLogEventBytesCalculation(t *testing.T) {
	testCases := []struct {
		name        string
		message     string
		description string
	}{
		{
			name:        "ShortMessage",
			message:     "Hello World",
			description: "Short message byte calculation",
		},
		{
			name:        "EmptyMessage",
			message:     "",
			description: "Empty message byte calculation",
		},
		{
			name:        "UnicodeMessage",
			message:     "Hello 世界 🌍",
			description: "Unicode message byte calculation",
		},
		{
			name:        "MaxSizeMessage",
			message:     strings.Repeat("A", msgSizeLimit),
			description: "Maximum size message byte calculation",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logEvent := newLogEvent(time.Now(), tc.message, nil)
			
			// The event bytes should include the message length plus the per-event header bytes
			expectedBytes := len(tc.message) + perEventHeaderBytes
			assert.Equal(t, expectedBytes, logEvent.eventBytes, 
				"Event bytes should be message length plus per-event header bytes")
		})
	}
}

// TestBatchSizeLimits tests that batches respect the size limits with corrected header calculation
func TestBatchSizeLimits(t *testing.T) {
	t.Run("BatchCapacityWithCorrectedHeaders", func(t *testing.T) {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)
		
		// Create a message that uses most of the available space
		messageSize := msgSizeLimit - 100 // Leave some room for testing
		message := strings.Repeat("A", messageSize)
		event := newLogEvent(time.Now(), message, nil)
		
		// The batch should have space for this event
		assert.True(t, batch.hasSpace(event.eventBytes), 
			"Batch should have space for event with corrected header calculation")
		
		// Add the event
		batch.append(event)
		
		// Now test if we can add another small event
		smallEvent := newLogEvent(time.Now(), "small", nil)
		canAddSmall := batch.hasSpace(smallEvent.eventBytes)
		
		// Calculate expected remaining space
		usedSpace := event.eventBytes
		remainingSpace := reqSizeLimit - usedSpace
		
		if remainingSpace >= smallEvent.eventBytes {
			assert.True(t, canAddSmall, "Should be able to add small event if space permits")
		} else {
			assert.False(t, canAddSmall, "Should not be able to add small event if no space")
		}
	})
}

// TestTruncationEdgeCases tests edge cases in message truncation
func TestTruncationEdgeCases(t *testing.T) {
	logger := testutil.NewNopLogger()
	target := Target{Group: "testGroup", Stream: "testStream"}
	conv := newConverter(logger, target)

	t.Run("MessageExactlyTruncatedSuffixLength", func(t *testing.T) {
		// Create a message that's exactly the length of the truncated suffix
		message := strings.Repeat("A", len(truncatedSuffix))
		event := newStubLogEvent(message, time.Now())
		
		logEvent := conv.convert(event)
		
		// Should not be truncated since it's well under the limit
		assert.Equal(t, message, logEvent.message, "Short message should not be truncated")
		assert.False(t, strings.HasSuffix(logEvent.message, truncatedSuffix), 
			"Short message should not have truncated suffix")
	})

	t.Run("MessageShorterThanTruncatedSuffix", func(t *testing.T) {
		// Create a message shorter than the truncated suffix
		message := "Hi"
		event := newStubLogEvent(message, time.Now())
		
		logEvent := conv.convert(event)
		
		// Should not be truncated
		assert.Equal(t, message, logEvent.message, "Very short message should not be truncated")
		assert.False(t, strings.HasSuffix(logEvent.message, truncatedSuffix), 
			"Very short message should not have truncated suffix")
	})

	t.Run("TruncatedMessageStructure", func(t *testing.T) {
		// Create a message that will be truncated
		originalMessage := strings.Repeat("ABCDEFGHIJ", msgSizeLimit/5) // Much larger than limit
		event := newStubLogEvent(originalMessage, time.Now())
		
		logEvent := conv.convert(event)
		
		// Verify the structure of the truncated message
		assert.Equal(t, msgSizeLimit, len(logEvent.message), "Truncated message should be exactly msgSizeLimit bytes")
		assert.True(t, strings.HasSuffix(logEvent.message, truncatedSuffix), "Should end with truncated suffix")
		
		// Verify the prefix is from the original message
		prefixLength := msgSizeLimit - len(truncatedSuffix)
		expectedPrefix := originalMessage[:prefixLength]
		actualPrefix := logEvent.message[:prefixLength]
		assert.Equal(t, expectedPrefix, actualPrefix, "Prefix should match original message")
	})
}

// TestBackwardCompatibility ensures the changes don't break existing functionality
func TestBackwardCompatibility(t *testing.T) {
	logger := testutil.NewNopLogger()
	target := Target{Group: "testGroup", Stream: "testStream"}
	conv := newConverter(logger, target)

	t.Run("NormalMessageProcessing", func(t *testing.T) {
		// Test that normal message processing still works
		message := "This is a normal log message"
		timestamp := time.Now()
		event := newStubLogEvent(message, timestamp)
		
		logEvent := conv.convert(event)
		
		assert.Equal(t, message, logEvent.message, "Normal message should be unchanged")
		assert.Equal(t, timestamp, logEvent.timestamp, "Timestamp should be preserved")
	})

	t.Run("TimestampHandling", func(t *testing.T) {
		// Test that timestamp handling still works correctly
		conv := newConverter(logger, target)
		validTime := time.Now()
		conv.lastValidTime = validTime
		
		// Event with no timestamp should use last valid time
		event := newStubLogEvent("Test message", time.Time{})
		logEvent := conv.convert(event)
		
		assert.Equal(t, validTime, logEvent.timestamp, "Should use last valid time when no timestamp provided")
	})
}
