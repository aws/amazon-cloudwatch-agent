// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
)

const (
	statusResetInterval = 5 * time.Minute
	statusHandlerID     = "cloudwatchagent.StatusCodeHandler"
)

var (
	statusCodeProviderSingleton *StatusCodeProvider
	StatusCodeProviderOnce      sync.Once
)

// StatusCodeProvider handles processing of status codes and maintains stats.
type StatusCodeProvider struct {
	currentStats   map[string]*[5]int
	mu             sync.RWMutex
	statusCodeChan chan statusCodeEntry
	stopChan       chan struct{}
	resetTicker    *time.Ticker
	completedStats chan agent.Stats
}

type statusCodeEntry struct {
	operation  string
	statusCode int
}

func GetStatusCodeStatsProvider() *StatusCodeProvider {
	StatusCodeProviderOnce.Do(func() {
		provider := &StatusCodeProvider{
			currentStats:   make(map[string]*[5]int),
			statusCodeChan: make(chan statusCodeEntry, 1000),
			stopChan:       make(chan struct{}),
			resetTicker:    time.NewTicker(statusResetInterval),
			completedStats: make(chan agent.Stats, 1), // buffered channel
		}
		provider.startProcessing()
		statusCodeProviderSingleton = provider
	})
	return statusCodeProviderSingleton
}

func (sp *StatusCodeProvider) startProcessing() {
	go func() {
		for {
			select {
			case entry := <-sp.statusCodeChan:
				sp.processStatusCode(entry)
			case <-sp.resetTicker.C:
				sp.RotateStats()
			case <-sp.stopChan:
				sp.resetTicker.Stop()
				return
			}
		}
	}()
}

func (sp *StatusCodeProvider) EnqueueStatusCode(operation string, statusCode int) {
	fmt.Println("Below is the operation name we are enqueueing for statuscode and its code")
	fmt.Println(operation, statusCode)
	sp.statusCodeChan <- statusCodeEntry{operation: operation, statusCode: statusCode}
}

func (sp *StatusCodeProvider) processStatusCode(entry statusCodeEntry) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	stats, exists := sp.currentStats[entry.operation]
	if !exists {
		stats = &[5]int{}
		sp.currentStats[entry.operation] = stats
	}

	switch entry.statusCode {
	case 200:
		stats[0]++
	case 400:
		stats[1]++
	case 408:
		stats[2]++
	case 413:
		stats[3]++
	case 429:
		stats[4]++
	}
}

func (sp *StatusCodeProvider) RotateStats() {
	sp.mu.Lock()
	newStats := agent.Stats{
		StatusCodes: make(map[string][5]int, len(sp.currentStats)),
	}
	for op, stats := range sp.currentStats {
		newStats.StatusCodes[op] = *stats
	}
	sp.currentStats = make(map[string]*[5]int)
	sp.mu.Unlock()

	select {
	case existingStats := <-sp.completedStats:
		existingStats.Merge(newStats)
		newStats = existingStats
	default:
	}

	sp.completedStats <- newStats
}

func (sp *StatusCodeProvider) Stats(_ string) agent.Stats {
	select {
	case stats := <-sp.completedStats:
		return stats
	default:
		return agent.Stats{}
	}
}

type StatusCodeHandler struct {
	StatusCodeProvider *StatusCodeProvider
	filter             agent.OperationsFilter
}

func NewStatusCodeHandler(provider *StatusCodeProvider, filter agent.OperationsFilter) *StatusCodeHandler {
	return &StatusCodeHandler{
		StatusCodeProvider: provider,
		filter:             filter,
	}
}

func (h *StatusCodeHandler) HandleResponse(ctx context.Context, r *http.Response) {
	operation := awsmiddleware.GetOperationName(ctx)
	fmt.Println("Thissss-----here is the statuscode:")
	fmt.Println(operation)
	if !h.filter.IsAllowed(operation) {
		fmt.Println("This operation is not allowed!!!!!!!!!")
		return
	}
	fmt.Println("This operation is allowed!!!!!")

	operation = agent.GetShortOperationName(operation)
	if operation == "" {
		return
	}

	h.StatusCodeProvider.EnqueueStatusCode(operation, r.StatusCode)
}

func (h *StatusCodeHandler) ID() string {
	return statusHandlerID
}

func (h *StatusCodeHandler) Position() awsmiddleware.HandlerPosition {
	return awsmiddleware.After
}
