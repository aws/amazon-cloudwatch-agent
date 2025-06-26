// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logscommon

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSizeConstants(t *testing.T) {
	// Test that all size limits are consistent for 1MiB
	assert.Equal(t, MaxNominalEventSize, 1048576, "MaxNominalEventSize should be 1MiB")
	assert.Equal(t, MaxEffectiveEventSize, 1048376, "MaxEffectiveEventSize should be 1MiB - 200B")
	assert.Equal(t, CwLogsHeaderBytes, 200, "Header bytes should be 200")
	assert.Equal(t, ReadBufferSize, MaxEffectiveEventSize, "ReadBufferSize should match MaxEffectiveEventSize")
}

func TestValidateEventSize(t *testing.T) {
	// Test event within limit
	smallEvent := strings.Repeat("a", 1000)
	result := ValidateEventSize(smallEvent)
	assert.Equal(t, smallEvent, result, "Small event should not be modified")

	// Test event exactly at limit
	exactEvent := strings.Repeat("b", MaxEffectiveEventSize)
	result = ValidateEventSize(exactEvent)
	assert.Equal(t, exactEvent, result, "Event at exact limit should not be modified")

	// Test event over limit
	largeEvent := strings.Repeat("c", MaxEffectiveEventSize+100)
	result = ValidateEventSize(largeEvent)
	assert.True(t, len(result) <= MaxEffectiveEventSize, "Large event should be truncated")
	assert.True(t, strings.HasSuffix(result, DefaultTruncateSuffix), "Truncated event should have suffix")
}

func TestCalculateEffectiveSize(t *testing.T) {
	// Test calculation
	nominalSize := 1000000
	effectiveSize := CalculateEffectiveSize(nominalSize)
	assert.Equal(t, nominalSize-CwLogsHeaderBytes, effectiveSize, "Effective size calculation should be correct")
}

func TestNoTinyEvents(t *testing.T) {
	// Ensure we never create events smaller than a reasonable minimum
	// This test validates the fix for the 2-byte event bug
	testSizes := []int{
		MaxEffectiveEventSize - 1,
		MaxEffectiveEventSize,
		MaxEffectiveEventSize + 1,
		MaxEffectiveEventSize + 2,
		MaxEffectiveEventSize + 100,
	}

	for _, size := range testSizes {
		event := strings.Repeat("x", size)
		result := ValidateEventSize(event)
		
		// Should never create tiny events
		assert.True(t, len(result) >= 1000 || len(result) == size, 
			"Event size %d should not create tiny result: got %d bytes", size, len(result))
	}
}
