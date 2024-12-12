// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package provider_test

import (
	"sync"
	"testing"
	"time"

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
	instance1 := provider.GetStatusCodeStatsProvider()
	instance2 := provider.GetStatusCodeStatsProvider()

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

func TestStatsResetRace(t *testing.T) {
	sp := provider.GetStatusCodeStatsProvider()

	// Pre-populate some stats through the normal channel
	sp.EnqueueStatusCode("op1", 200)
	sp.EnqueueStatusCode("op2", 400)

	// Give time for the stats to be processed
	time.Sleep(10 * time.Millisecond)

	// Trigger a rotation to get some stats in the completedStats channel
	sp.RotateStats()

	var wg sync.WaitGroup
	wg.Add(3)

	// Goroutine 1: Continuously call the Stats method
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			stats := sp.Stats("")
			if stats.StatusCodes != nil {
				total := 0
				for _, counts := range stats.StatusCodes {
					for _, count := range counts {
						total += count
					}
				}
				assert.Greater(t, total, 0, "Should have some status codes counted")
			}
		}
	}()

	// Goroutine 2: Add new status codes
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			sp.EnqueueStatusCode("op3", 200)
		}
	}()

	// Goroutine 3: Trigger rotations
	go func() {
		defer wg.Done()
		for i := 0; i < 3; i++ {
			time.Sleep(1 * time.Millisecond)
			sp.RotateStats()
		}
	}()

	wg.Wait()
}
