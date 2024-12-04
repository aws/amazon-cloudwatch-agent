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
	resetTimer       *time.Timer
	filter           agent.OperationsFilter
	mu               sync.Mutex
}

// GetStatusCodeStats retrieves or initializes the singleton StatusCodeHandler.
func GetStatusCodeStats(filter interface{}) *StatusCodeHandler {
	log.Println("Creating a handler")
	statusCodeStatsOnce.Do(func() {
		handler := &StatusCodeHandler{
			statsByOperation: make(map[string]*[5]int),
		}

		if opsFilter, ok := filter.(agent.OperationsFilter); ok {
			handler.filter = opsFilter
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
		for key := range h.statsByOperation {
			delete(h.statsByOperation, key)
		}
		h.startResetTimer()
	})
}

// HandleRequest is a no-op for the StatusCodeHandler.
func (h *StatusCodeHandler) HandleRequest(ctx context.Context, _ *http.Request) {}

// HandleResponse processes the HTTP response to update status code stats.
func (h *StatusCodeHandler) HandleResponse(ctx context.Context, r *http.Response) {
	// Extract the operation name
	operation := awsmiddleware.GetOperationName(ctx)

	if !h.filter.IsAllowed(operation) {
		return
	}

	operation = GetShortOperationName(operation)
	if operation == "" {
		return
	}

	statusCode := r.StatusCode

	h.mu.Lock()
	defer h.mu.Unlock()

	stats, exists := h.statsByOperation[operation]
	if !exists {
		stats = &[5]int{}
		h.statsByOperation[operation] = stats
	}

	h.updateStatusCodeCount(stats, statusCode, operation)
}

// Helper function to update the status code counts
func (h *StatusCodeHandler) updateStatusCodeCount(stats *[5]int, statusCode int, operation string) {
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
		return
	}
}

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
	// Lock mutex to safely access statsByOperation
	h.mu.Lock()
	defer h.mu.Unlock()

	statusCodeMap := make(map[string][5]int)

	for operation, stats := range h.statsByOperation {
		statusCodeMap[operation] = *stats
	}

	return agent.Stats{
		StatusCodes: statusCodeMap,
	}
}
