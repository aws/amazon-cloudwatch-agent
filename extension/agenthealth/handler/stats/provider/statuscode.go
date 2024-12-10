// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"log"
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
	StatsByOperation map[string]*[5]int
	ResetTimer       *time.Timer
	statusCodeChan   chan statusCodeEntry
	stopChan         chan struct{}
	ShouldResetStats bool
	Mu               sync.RWMutex
}

// statusCodeEntry represents a status code and its associated operation.
type statusCodeEntry struct {
	operation  string
	statusCode int
}

// StatusCodeHandler is the handler that uses the StatusCodeProvider for processing.
type StatusCodeHandler struct {
	StatusCodeProvider *StatusCodeProvider
	filter             agent.OperationsFilter
}

// StatusCodeProvider methods

func GetStatusCodeStatsProvider() *StatusCodeProvider {
	StatusCodeProviderOnce.Do(func() {
		provider := &StatusCodeProvider{
			StatsByOperation: make(map[string]*[5]int),
			statusCodeChan:   make(chan statusCodeEntry, 1000),
			stopChan:         make(chan struct{}),
		}

		provider.startResetTimer()
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
			case <-sp.stopChan:
				return
			}
		}
	}()
}

func (sp *StatusCodeProvider) EnqueueStatusCode(operation string, statusCode int) {
	select {
	case sp.statusCodeChan <- statusCodeEntry{operation: operation, statusCode: statusCode}:
		log.Printf("Successfully enqueued status code %d for operation %s", statusCode, operation)
	default:
		log.Printf("Warning: StatusCode channel full, dropping status code %d for operation %s", statusCode, operation)
	}
}

func (sp *StatusCodeProvider) processStatusCode(entry statusCodeEntry) {
	sp.Mu.Lock()
	defer sp.Mu.Unlock()

	stats, exists := sp.StatsByOperation[entry.operation]
	if !exists {
		stats = &[5]int{}
		sp.StatsByOperation[entry.operation] = stats
	}
	sp.updateStatusCodeCount(stats, entry.statusCode)
}

func (sp *StatusCodeProvider) updateStatusCodeCount(stats *[5]int, statusCode int) {
	switch statusCode {
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

func (sp *StatusCodeProvider) startResetTimer() {
	sp.ResetTimer = time.AfterFunc(statusResetInterval, func() {
		sp.Mu.Lock()
		sp.ShouldResetStats = true
		sp.Mu.Unlock()
	})
}

func (sp *StatusCodeProvider) Stats(_ string) agent.Stats {
	sp.Mu.Lock()
	defer sp.Mu.Unlock()

	statusCodeMap := make(map[string][5]int)
	if sp.ShouldResetStats {
		for op, stats := range sp.StatsByOperation {
			statusCodeMap[op] = *stats
		}
		sp.StatsByOperation = make(map[string]*[5]int)
		sp.ShouldResetStats = false
	} else {
		for op, stats := range sp.StatsByOperation {
			statusCodeMap[op] = *stats
		}
	}

	return agent.Stats{
		StatusCodes: statusCodeMap,
	}
}

// StatusCodeHandler methods

func NewStatusCodeHandler(provider *StatusCodeProvider, filter agent.OperationsFilter) *StatusCodeHandler {
	return &StatusCodeHandler{StatusCodeProvider: provider, filter: filter}
}

func (h *StatusCodeHandler) ID() string {
	return statusHandlerID
}

func (h *StatusCodeHandler) Position() awsmiddleware.HandlerPosition {
	return awsmiddleware.After
}

func (h *StatusCodeHandler) HandleResponse(ctx context.Context, r *http.Response) {
	operation := awsmiddleware.GetOperationName(ctx)
	if !h.filter.IsAllowed(operation) {
		return
	}

	operation = agent.GetShortOperationName(operation)
	if operation == "" {
		return
	}

	h.StatusCodeProvider.EnqueueStatusCode(operation, r.StatusCode)
}