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

	operation = GetShortOperationName(operation)
	statusCode := r.StatusCode
	log.Printf("Received status code: %d for operation: %s", statusCode, operation)

	h.mu.Lock()
	defer h.mu.Unlock()

	value, loaded := h.statsByOperation.LoadOrStore(operation, &[5]int{})
	if !loaded {
		log.Printf("Initializing stats for operation: %s", operation)
	}
	stats := value.(*[5]int)

	h.updateStatusCodeCount(stats, statusCode, operation)

	h.statsByOperation.Store(operation, stats)
	log.Printf("Updated stats for operation '%s': 200=%d, 400=%d, 408=%d, 413=%d, 429=%d", operation, stats[0], stats[1], stats[2], stats[3], stats[4])

	log.Println("Complete status code map:")
	h.statsByOperation.Range(func(key, value interface{}) bool {
		log.Print("Printing all stats by operations map")

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
		log.Printf("Incremented 200 count for operation: %s. New 200=%d", operation, stats[0])
	case 400:
		stats[1]++
		log.Printf("Incremented 400 count for operation: %s. New 400=%d", operation, stats[1])
	case 408:
		stats[2]++
		log.Printf("Incremented 408 count for operation: %s. New 408=%d", operation, stats[2])
	case 413:
		stats[3]++
		log.Printf("Incremented 413 count for operation: %s. New 413=%d", operation, stats[3])
	case 429:
		stats[4]++
		log.Printf("Incremented 429 count for operation: %s. New 429=%d", operation, stats[4])
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
		return "sts"
	default:
		return operation
	}
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
	h.mu.Lock()
	defer h.mu.Unlock()

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
