package provider

import (
	"context"
	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
)

const (
	statusResetInterval = 5 * time.Minute
	statusHandlerID     = "cloudwatchagent.StatusCodeHandler"
)

var (
	statsProviderSingleton agent.StatsProvider
	statsProviderOnce      sync.Once
)

// SingletonStatsProvider manages a collection of statistics.
type SingletonStatsProvider struct {
	mu              sync.Mutex
	statusCodeStats map[string][5]int
}

// StatusCodeHandler provides monitoring for status codes per operation.
type StatusCodeHandler struct {
	statsProvider *SingletonStatsProvider
	filter        agent.OperationsFilter
	resetTimer    *time.Timer
	mu            sync.Mutex
}

// NewStatusCodeHandler creates a new instance of StatusCodeHandler.
func NewStatusCodeHandler(filter agent.OperationsFilter) *StatusCodeHandler {
	provider := GetStatsProvider().(*SingletonStatsProvider) // Get the singleton provider.
	handler := &StatusCodeHandler{
		statsProvider: provider,
		filter:        filter,
	}
	handler.startResetTimer()
	return handler
}

// GetStatsProvider retrieves the singleton instance of the `agent.StatsProvider`.
func GetStatsProvider() agent.StatsProvider {
	statsProviderOnce.Do(func() {
		statsProviderSingleton = &SingletonStatsProvider{
			statusCodeStats: make(map[string][5]int),
		}
	})
	return statsProviderSingleton
}

// Stats returns the current statistics for a given operation.
func (p *SingletonStatsProvider) Stats(operation string) agent.Stats {
	p.mu.Lock()
	defer p.mu.Unlock()

	log.Println("Operation below:")
	log.Println(operation)
	statusCodeMap := make(map[string][5]int, len(p.statusCodeStats))
	for op, stats := range p.statusCodeStats {
		statusCodeMap[op] = stats
	}

	log.Println("Status code map:")
	log.Println(statusCodeMap)
	return agent.Stats{
		StatusCodes: statusCodeMap,
	}
}

// UpdateStats updates the statistics for a given operation.
func (p *SingletonStatsProvider) UpdateStats(operation string, stats [5]int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.statusCodeStats[operation] = stats
}

// startResetTimer initializes a reset timer to clear stats every 5 minutes.
func (h *StatusCodeHandler) startResetTimer() {
	h.resetTimer = time.AfterFunc(statusResetInterval, func() {
		h.mu.Lock()
		defer h.mu.Unlock()

		h.statsProvider.mu.Lock()
		h.statsProvider.statusCodeStats = make(map[string][5]int)
		h.statsProvider.mu.Unlock()

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

	// Get or initialize stats
	h.statsProvider.mu.Lock()
	stats := h.statsProvider.statusCodeStats[operation]
	h.statsProvider.mu.Unlock()

	h.updateStatusCodeCount(&stats, statusCode)

	// Update the singleton stats provider
	h.statsProvider.UpdateStats(operation, stats)

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

func GetShortOperationName(operation string) string {
	switch operation {
	case "PutRetentionPolicy":
		return "prp"
	case "DescribeInstances":
		return "di"
	case "DescribeTasks":
		return "dt"
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
