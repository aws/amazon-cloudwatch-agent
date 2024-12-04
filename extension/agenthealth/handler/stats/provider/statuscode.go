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
	statusCodeSingleton *StatusCodeHandler
	statusCodeStatsOnce sync.Once
)

// StatusCodeHandler provides monitoring for status codes per operation.
type StatusCodeHandler struct {
	statsByOperation map[string]*[5]int
	mu               sync.Mutex
	resetTimer       *time.Timer
	filter           agent.OperationsFilter
}

// GetStatusCodeStats retrieves or initializes the singleton StatusCodeHandler.
func GetStatusCodeStats(filter agent.OperationsFilter) *StatusCodeHandler {
	statusCodeStatsOnce.Do(func() {
		handler := &StatusCodeHandler{
			statsByOperation: make(map[string]*[5]int),
			filter:           filter,
		}
		handler.startResetTimer()
		statusCodeSingleton = handler
	})
	return statusCodeSingleton
}

// startResetTimer initializes a reset timer to clear stats every 5 minutes.
func (h *StatusCodeHandler) startResetTimer() {
	h.resetTimer = time.AfterFunc(statusResetInterval, func() {
		h.mu.Lock()
		defer h.mu.Unlock()

		h.statsByOperation = make(map[string]*[5]int)
		log.Println("Status code stats reset.")
		h.startResetTimer()
	})
}

// HandleRequest is a no-op for the StatusCodeHandler.
func (h *StatusCodeHandler) HandleRequest(ctx context.Context, _ *http.Request) {}

// HandleResponse processes the HTTP response to update status code stats.
func (h *StatusCodeHandler) HandleResponse(ctx context.Context, r *http.Response) {
	operation := awsmiddleware.GetOperationName(ctx)
	if operation == "" {
		log.Println("No operation name found in the context")
		return
	} else if !h.filter.IsAllowed(operation) {
		log.Printf("Operation %s is not allowed", operation)
		return
	} else {
		log.Printf("Processing response for operation: %s", operation)
	}

	operation = GetShortOperationName(operation)
	statusCode := r.StatusCode
	log.Printf("Received status code: %d for operation: %s", statusCode, operation)

	h.mu.Lock()
	defer h.mu.Unlock()

	stats, exists := h.statsByOperation[operation]
	if !exists {
		stats = &[5]int{}
		h.statsByOperation[operation] = stats
		log.Printf("Initializing stats for operation: %s", operation)
	}

	h.updateStatusCodeCount(stats, statusCode)

	log.Printf("Updated stats for operation '%s': 200=%d, 400=%d, 408=%d, 413=%d, 429=%d", operation, stats[0], stats[1], stats[2], stats[3], stats[4])
}

// updateStatusCodeCount updates the count for a given status code.
func (h *StatusCodeHandler) updateStatusCodeCount(stats *[5]int, statusCode int) {
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

// GetShortOperationName returns a shortened name for known operations.
func GetShortOperationName(operation string) string {
	switch operation {
	case "PutRetentionPolicy":
		return "prp"
	case "DescribeInstances":
		return "di"
	case "DescribeTags":
		return "dt"
	case "DescribeVolumes":
		return "dv"
	case "DescribeContainerInstances":
		return "dci"
	case "DescribeServices":
		return "ds"
	case "DescribeTaskDefinition":
		return "dtd"
	case "ListServices":
		return "ls"
	case "ListTasks":
		return "lt"
	case "CreateLogGroup":
		return "clg"
	case "CreateLogStream":
		return "cls"
	default:
		return ""
	}
}

// ID returns the unique identifier for the handler.
func (h *StatusCodeHandler) ID() string {
	return statusHandlerID
}

// Position specifies the handler's position in the middleware chain.
func (h *StatusCodeHandler) Position() awsmiddleware.HandlerPosition {
	return awsmiddleware.After
}

// Stats implements the `Stats` method required by the `agent.StatsProvider` interface.
func (h *StatusCodeHandler) Stats(operation string) agent.Stats {
	h.mu.Lock()
	defer h.mu.Unlock()

	statusCodeMap := make(map[string][5]int, len(h.statsByOperation))
	for op, stats := range h.statsByOperation {
		statusCodeMap[op] = *stats
	}

	return agent.Stats{
		StatusCodes: statusCodeMap,
	}
}
