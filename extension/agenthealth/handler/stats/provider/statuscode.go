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
	channelBufferSize   = 100
)

var (
	statusCodeProviderSingleton *StatsProvider
	statsProviderOnce           sync.Once
)

// StatsProvider handles processing of status codes and maintains stats.
type StatsProvider struct {
	statsByOperation map[string]*[5]int
	resetTimer       *time.Timer
	filter           agent.OperationsFilter
	statusCodeChan   chan statusCodeEntry
	stopChan         chan struct{}
	mu               sync.Mutex
	wg               sync.WaitGroup
}

// statusCodeEntry represents a status code and its associated operation.
type statusCodeEntry struct {
	operation  string
	statusCode int
}

// GetStatsProvider initializes and retrieves the singleton StatsProvider.
func GetStatsProvider(filter interface{}) *StatsProvider {
	statsProviderOnce.Do(func() {
		provider := &StatsProvider{
			statsByOperation: make(map[string]*[5]int),
			statusCodeChan:   make(chan statusCodeEntry, channelBufferSize),
			stopChan:         make(chan struct{}),
		}

		if opsFilter, ok := filter.(agent.OperationsFilter); ok {
			provider.filter = opsFilter
		}
		provider.startResetTimer()
		provider.startProcessing()
		statusCodeProviderSingleton = provider
	})
	return statusCodeProviderSingleton
}

// startProcessing begins processing status codes from the channel.
func (sp *StatsProvider) startProcessing() {
	sp.wg.Add(1)
	go func() {
		defer sp.wg.Done()
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

// Stop signals the StatsProvider to stop processing and waits for cleanup.
func (sp *StatsProvider) Stop() {
	close(sp.stopChan)
	sp.wg.Wait()
}

// EnqueueStatusCode adds a status code entry to the channel.
func (sp *StatsProvider) EnqueueStatusCode(operation string, statusCode int) {
	sp.statusCodeChan <- statusCodeEntry{operation: operation, statusCode: statusCode}
}

// processStatusCode updates the stats map for the given status code entry.
func (sp *StatsProvider) processStatusCode(entry statusCodeEntry) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	stats, exists := sp.statsByOperation[entry.operation]
	if !exists {
		stats = &[5]int{}
		sp.statsByOperation[entry.operation] = stats
	}
	sp.updateStatusCodeCount(stats, entry.statusCode)
}

// updateStatusCodeCount updates the count for the specific status code.
func (sp *StatsProvider) updateStatusCodeCount(stats *[5]int, statusCode int) {
	log.Printf("Updating stats for status code: %d\n", statusCode)
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
		log.Printf("Ignored status code: %d\n", statusCode)
	}
}

// startResetTimer initializes a reset timer to clear stats periodically.
func (sp *StatsProvider) startResetTimer() {
	log.Println("Starting stats reset timer...")
	sp.resetTimer = time.AfterFunc(statusResetInterval, func() {
		sp.mu.Lock()
		defer sp.mu.Unlock()
		log.Println("Resetting stats...")
		for key := range sp.statsByOperation {
			delete(sp.statsByOperation, key)
		}
		sp.startResetTimer()
	})
}

// StatusCodeHandler is the handler that uses the StatsProvider for processing.
type StatusCodeHandler struct {
	statsProvider *StatsProvider
}

func (h *StatusCodeHandler) ID() string {
	return statusHandlerID
}

func (h *StatusCodeHandler) Position() awsmiddleware.HandlerPosition {
	return awsmiddleware.After
}

// NewStatusCodeHandler creates a new handler with the given StatsProvider.
func NewStatusCodeHandler(provider *StatsProvider) *StatusCodeHandler {
	log.Println("Creating new StatusCodeHandler...")
	return &StatusCodeHandler{statsProvider: provider}
}

// HandleRequest is a no-op for the StatusCodeHandler.
func (h *StatusCodeHandler) HandleRequest(ctx context.Context, _ *http.Request) {}

// HandleResponse enqueues the status code into the StatsProvider's channel.
func (h *StatusCodeHandler) HandleResponse(ctx context.Context, r *http.Response) {
	operation := awsmiddleware.GetOperationName(ctx)
	if !h.statsProvider.filter.IsAllowed(operation) {
		return
	}

	operation = GetShortOperationName(operation)
	if operation == "" {
		return
	}

	h.statsProvider.EnqueueStatusCode(operation, r.StatusCode)
}

// GetShortOperationName maps long operation names to short ones.
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

// Stats returns the aggregated stats for operations.
func (sp *StatsProvider) Stats(operation string) agent.Stats {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	statusCodeMap := make(map[string][5]int)
	for op, stats := range sp.statsByOperation {
		statusCodeMap[op] = *stats
	}
	return agent.Stats{
		StatusCodes: statusCodeMap,
	}
}
