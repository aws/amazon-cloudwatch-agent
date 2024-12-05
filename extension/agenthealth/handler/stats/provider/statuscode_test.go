// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package provider_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/provider"
)

func TestNewHandlers(t *testing.T) {
	logger := zap.NewNop() // Use a no-op logger for testing
	cfg := agent.StatsConfig{
		Operations: []string{"TestOperation"},
	}

	t.Run("Only StatusCodeEnabled", func(t *testing.T) {
		requestHandlers, responseHandlers := stats.NewHandlers(logger, cfg, true, false)

		assert.Nil(t, requestHandlers, "Request handlers should not be nil")
		assert.NotNil(t, responseHandlers, "Response handlers should not be nil")
		assert.Len(t, requestHandlers, 0, "There should be 0 request handlers")
		assert.Len(t, responseHandlers, 1, "There should be 1 response handler")

		assert.IsType(t, &provider.StatusCodeHandler{}, responseHandlers[0], "First response handler should be StatusCodeHandler")
	})

	t.Run("Only AgentStatsEnabled", func(t *testing.T) {
		requestHandlers, responseHandlers := stats.NewHandlers(logger, cfg, false, true)

		assert.NotNil(t, requestHandlers, "Request handlers should not be nil")
		assert.NotNil(t, responseHandlers, "Response handlers should not be nil")
		assert.GreaterOrEqual(t, len(requestHandlers), 2, "There should be at least 2 request handlers")
		assert.GreaterOrEqual(t, len(responseHandlers), 1, "There should be at least 1 response handler")
	})

	t.Run("Both Enabled", func(t *testing.T) {
		requestHandlers, responseHandlers := stats.NewHandlers(logger, cfg, true, true)

		assert.NotNil(t, requestHandlers, "Request handlers should not be nil")
		assert.NotNil(t, responseHandlers, "Response handlers should not be nil")
		assert.GreaterOrEqual(t, len(requestHandlers), 2, "There should be at least 3 request handlers")
		assert.GreaterOrEqual(t, len(responseHandlers), 2, "There should be at least 2 response handlers")
	})

	t.Run("Neither Enabled", func(t *testing.T) {
		requestHandlers, responseHandlers := stats.NewHandlers(logger, cfg, false, false)

		assert.Nil(t, requestHandlers, "Request handlers should be nil")
		assert.Nil(t, responseHandlers, "Response handlers should be nil")
	})
}

func TestSingleton(t *testing.T) {
	instance1 := provider.GetStatusCodeStatsProvider(nil)
	instance2 := provider.GetStatusCodeStatsProvider(nil)

	if instance1 != instance2 {
		t.Errorf("Expected both instances to be the same, but they are different")
	}

	instance1.EnqueueStatusCode("DescribeInstances", 200)
	stats1 := instance1.Stats("")
	stats2 := instance2.Stats("")

	if stats1.StatusCodes["DescribeInstances"][0] != stats2.StatusCodes["DescribeInstances"][0] {
		t.Errorf("Expected the state to be the same across instances, but it differs")
	}
}
