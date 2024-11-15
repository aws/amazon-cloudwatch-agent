package provider

import (
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusCodeHandler(t *testing.T) {
	// Create a mock OperationsFilter
	filter := agent.NewStatusCodeOperationsFilter()

	// Retrieve the handler with the mock filter
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
