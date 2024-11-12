package provider

import (
	"context"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
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
	mu               sync.Mutex
	resetTimer       *time.Timer
}

// GetStatusCodeStats retrieves or initializes the singleton StatusCodeHandler.
func GetStatusCodeStats() *StatusCodeHandler {
	statusCodeStatsOnce.Do(func() {
		handler := &StatusCodeHandler{}
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

		h.statsByOperation.Range(func(key, _ interface{}) bool {
			h.statsByOperation.Delete(key)
			return true
		})
		log.Println("Status code stats reset.")
		h.startResetTimer()
	})
}

// HandleRequest is a no-op for the StatusCodeHandler.
func (h *StatusCodeHandler) HandleRequest(ctx context.Context, _ *http.Request) {}

// HandleResponse processes the HTTP response to update status code stats.
func (h *StatusCodeHandler) HandleResponse(ctx context.Context, r *http.Response) {
	// Extract the operation name
	operation := awsmiddleware.GetOperationName(ctx)
	if operation == "" {
		log.Println("No operation name found in the context")
	} else {
		log.Printf("Processing response for operation: %s", operation)
	}

	// Extract the status code
	statusCode := r.StatusCode
	log.Printf("Received status code: %d for operation: %s", statusCode, operation)

	h.mu.Lock()
	defer h.mu.Unlock()

	// Load or initialize stats for the operation
	value, loaded := h.statsByOperation.LoadOrStore(operation, &[2]int{})
	if !loaded {
		log.Printf("Initializing stats for operation: %s", operation)
	}
	stats := value.(*[2]int)

	// Update success or failure count
	if statusCode >= 200 && statusCode < 300 {
		stats[0]++
		log.Printf("Incremented success count for operation: %s. New Success=%d", operation, stats[0])
	} else {
		stats[1]++
		log.Printf("Incremented failure count for operation: %s. New Failures=%d", operation, stats[1])
	}

	// Store updated stats back in the map
	h.statsByOperation.Store(operation, stats)
	log.Printf("Updated stats for operation '%s': Success=%d, Failures=%d", operation, stats[0], stats[1])
}

// GetStats retrieves the success and failure counts for a specific operation.
func (h *StatusCodeHandler) GetStats(operation string) (successCount, failureCount int) {
	value, ok := h.statsByOperation.Load(operation)
	if !ok {
		return 0, 0
	}
	stats := value.(*[2]int)
	return stats[0], stats[1]
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
	value, ok := h.statsByOperation.Load(operation)
	if !ok {
		return agent.Stats{
			StatusCodes: map[string][2]int{},
		}
	}

	stats := value.(*[2]int)
	return agent.Stats{
		StatusCodes: map[string][2]int{
			operation: {stats[0], stats[1]},
		},
	}
}
