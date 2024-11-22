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
	statsByOperation sync.Map
	resetTimer       *time.Timer
	filter           agent.OperationsFilter
}

// GetStatusCodeStats retrieves or initializes the singleton StatusCodeHandler.
func GetStatusCodeStats(filter agent.OperationsFilter) *StatusCodeHandler {
	statusCodeStatsOnce.Do(func() {
		handler := &StatusCodeHandler{}
		handler.filter = filter
		handler.startResetTimer()
		statusCodeSingleton = handler
	})
	return statusCodeSingleton
}

// startResetTimer initializes a reset timer to clear stats every 5 minutes.
func (h *StatusCodeHandler) startResetTimer() {
	ticker := time.NewTicker(statusResetInterval)

	go func() {
		for range ticker.C {
			h.statsByOperation.Clear()
			log.Println("Status code stats reset.")
		}
	}()
}

// HandleRequest is a no-op for the StatusCodeHandler.
func (h *StatusCodeHandler) HandleRequest(ctx context.Context, _ *http.Request) {}

// HandleResponse processes the HTTP response to update status code stats.
func (h *StatusCodeHandler) HandleResponse(ctx context.Context, r *http.Response) {
	// Extract the operation name
	operation := awsmiddleware.GetOperationName(ctx)
	if !h.filter.IsAllowed(operation) {
		log.Printf("Operation %s is not allowed", operation)
		return
	} else {
		log.Printf("Processing response for operation: %s", operation)
	}

	operation = GetShortOperationName(operation)
	if operation == "" {
		return
	}
	statusCode := r.StatusCode

	value, loaded := h.statsByOperation.LoadOrStore(operation, &[5]int{})
	if !loaded {
		log.Printf("Initializing stats for operation: %s", operation)
	}
	stats := value.(*[5]int)

	h.updateStatusCodeCount(stats, statusCode, operation)

	h.statsByOperation.Store(operation, stats)

	h.statsByOperation.Range(func(key, value interface{}) bool {

		operation := key.(string)
		stats := value.(*[5]int)
		log.Printf("Operation: %s, 200=%d, 400=%d, 408=%d, 413=%d, 429=%d", operation, stats[0], stats[1], stats[2], stats[3], stats[4])
		return true
	})
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
		log.Printf("Received an untracked status code %d for operation: %s", statusCode, operation)
	}
}

func GetShortOperationName(operation string) string {
	switch operation {
	case "PutMetricData":
		return "pmd"
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
	case "AssumeRole":
		return "ar"
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

	statusCodeMap := make(map[string][5]int)

	h.statsByOperation.Range(func(key, value interface{}) bool {
		operation := key.(string)
		stats := value.(*[5]int)
		statusCodeMap[operation] = [5]int{stats[0], stats[1], stats[2], stats[3], stats[4]}
		return true
	})

	return agent.Stats{
		StatusCodes: statusCodeMap,
	}
}
