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
	statsByOperation map[string]*[5]int
	resetTimer       *time.Timer
	filter           agent.OperationsFilter
	statusCodeChan   chan statusCodeEntry
	stopChan         chan struct{}
	shouldResetStats bool
	mu               sync.Mutex
	wg               sync.WaitGroup
}

// statusCodeEntry represents a status code and its associated operation.
type statusCodeEntry struct {
	operation  string
	statusCode int
}

// GetStatusCodeStatusCodeProvider initializes and retrieves the singleton StatusCodeProvider.
func GetStatusCodeStatsProvider(filter interface{}) *StatusCodeProvider {
	StatusCodeProviderOnce.Do(func() {
		log.Println("Initializing StatusCodeProvider...")
		provider := &StatusCodeProvider{
			statsByOperation: make(map[string]*[5]int),
			statusCodeChan:   make(chan statusCodeEntry),
			stopChan:         make(chan struct{}),
		}

		if opsFilter, ok := filter.(agent.OperationsFilter); ok {
			log.Println("Operations filter applied.")
			provider.filter = opsFilter
		}
		provider.startResetTimer()
		provider.startProcessing()
		statusCodeProviderSingleton = provider
	})
	return statusCodeProviderSingleton
}

// startProcessing begins processing status codes from the channel.
func (sp *StatusCodeProvider) startProcessing() {
	log.Println("Starting status code processing...")
	sp.wg.Add(1)
	go func() {
		defer sp.wg.Done()
		for {
			select {
			case entry := <-sp.statusCodeChan:
				log.Printf("Processing status code: operation=%s, statusCode=%d\n", entry.operation, entry.statusCode)
				sp.processStatusCode(entry)
			case <-sp.stopChan:
				log.Println("Stopping status code processing.")
				return
			}
		}
	}()
}

// EnqueueStatusCode adds a status code entry to the channel.
func (sp *StatusCodeProvider) EnqueueStatusCode(operation string, statusCode int) {
	log.Printf("Enqueuing status code: operation=%s, statusCode=%d\n", operation, statusCode)
	sp.statusCodeChan <- statusCodeEntry{operation: operation, statusCode: statusCode}
}

// processStatusCode updates the stats map for the given status code entry.
func (sp *StatusCodeProvider) processStatusCode(entry statusCodeEntry) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	stats, exists := sp.statsByOperation[entry.operation]
	if !exists {
		log.Printf("Initializing stats for operation: %s\n", entry.operation)
		stats = &[5]int{}
		sp.statsByOperation[entry.operation] = stats
	}
	log.Println(entry.operation)
	sp.updateStatusCodeCount(stats, entry.statusCode)
}

// updateStatusCodeCount updates the count for the specific status code.
func (sp *StatusCodeProvider) updateStatusCodeCount(stats *[5]int, statusCode int) {
	log.Printf("Updating status code count: statusCode=%d\n", statusCode)
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
	default:
		log.Printf("Unknown status code encountered: %d\n", statusCode)
	}
	log.Printf(
		"Updated stats for operation ??: 200=%d, 400=%d, 408=%d, 413=%d, 429=%d",
		stats[0], stats[1], stats[2], stats[3], stats[4],
	)
}

// startResetTimer initializes a reset timer to clear stats periodically.
func (sp *StatusCodeProvider) startResetTimer() {
	log.Println("Starting reset timer...")
	sp.resetTimer = time.AfterFunc(statusResetInterval, func() {
		sp.mu.Lock()
		defer sp.mu.Unlock()
		log.Println("Resetting stats...")
		for key := range sp.statsByOperation {
			delete(sp.statsByOperation, key)
		}
		sp.shouldResetStats = true
		sp.startResetTimer()
	})
}

// StatusCodeHandler is the handler that uses the StatusCodeProvider for processing.
type StatusCodeHandler struct {
	StatusCodeProvider *StatusCodeProvider
}

func (h *StatusCodeHandler) ID() string {
	return statusHandlerID
}

func (h *StatusCodeHandler) Position() awsmiddleware.HandlerPosition {
	return awsmiddleware.After
}

// NewStatusCodeHandler creates a new handler with the given StatusCodeProvider.
func NewStatusCodeHandler(provider *StatusCodeProvider) *StatusCodeHandler {
	log.Println("Creating new StatusCodeHandler...")
	return &StatusCodeHandler{StatusCodeProvider: provider}
}

// HandleResponse enqueues the status code into the StatusCodeProvider's channel.
func (h *StatusCodeHandler) HandleResponse(ctx context.Context, r *http.Response) {
	operation := awsmiddleware.GetOperationName(ctx)
	if !h.StatusCodeProvider.filter.IsAllowed(operation) {
		log.Printf("Operation not allowed: %s\n", operation)
		return
	}

	operation = agent.GetShortOperationName(operation)
	if operation == "" {
		log.Println("Operation name is empty after shortening.")
		return
	}

	log.Printf("Handling response: operation=%s, statusCode=%d\n", operation, r.StatusCode)
	h.StatusCodeProvider.EnqueueStatusCode(operation, r.StatusCode)
}

// Stats returns the aggregated stats for operations.
func (sp *StatusCodeProvider) Stats(_ string) agent.Stats {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	statusCodeMap := make(map[string][5]int)

	if sp.shouldResetStats {
		log.Println("Reset stats flag detected. Capturing stats snapshot...")
		for op, stats := range sp.statsByOperation {
			statusCodeMap[op] = *stats
		}
		sp.shouldResetStats = false
	}

	return agent.Stats{
		StatusCodes: statusCodeMap,
	}
}
