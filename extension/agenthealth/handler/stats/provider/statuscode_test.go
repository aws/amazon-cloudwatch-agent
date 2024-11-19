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
	handler.statsByOperation.Store("pmd", &[5]int{1, 2, 0, 1, 0})
	stats := handler.Stats("pmd")
	expected := [5]int{1, 2, 0, 1, 0}
	actualStats := stats.StatusCodes["pmd"]
	assert.Equal(t, expected, actualStats, "Unexpected stats values for operation 'pmd'")
	assert.Contains(t, stats.StatusCodes, "pmd", "Status code map should contain 'pmd'")
	assert.Equal(t, expected, stats.StatusCodes["pmd"], "Stats for 'pmd' do not match")
}
