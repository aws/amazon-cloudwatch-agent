// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
)

func TestStatusCodeHandler(t *testing.T) {
	filter := agent.NewStatusCodeOperationsFilter()
	handler := GetStatusCodeStats(filter)
	require.NotNil(t, handler)

	// Locking to ensure thread-safe operations
	handler.mu.Lock()
	handler.statsByOperation["pmd"] = &[5]int{1, 2, 0, 1, 0}
	handler.mu.Unlock()

	// Retrieve stats after modification
	stats := handler.Stats("pmd")
	expected := [5]int{1, 2, 0, 1, 0}
	actualStats := stats.StatusCodes["pmd"]

	// Perform assertions
	assert.Equal(t, expected, actualStats, "Unexpected stats values for operation 'pmd'")
	assert.Contains(t, stats.StatusCodes, "pmd", "Status code map should contain 'pmd'")
	assert.Equal(t, expected, stats.StatusCodes["pmd"], "Stats for 'pmd' do not match")
}
