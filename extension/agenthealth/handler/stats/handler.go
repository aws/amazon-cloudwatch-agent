// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package stats

import (
	"context"
	"log"
	"net/http"
	"sync"

	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/client"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/provider"
)

const (
	handlerID           = "cloudwatchagent.AgentStats"
	headerKeyAgentStats = "X-Amz-Agent-Stats"
)

func NewHandlers(logger *zap.Logger, cfg agent.StatsConfig, statusCodeEnabled bool, agentStatsEnabled bool) ([]awsmiddleware.RequestHandler, []awsmiddleware.ResponseHandler) {
	var requestHandlers []awsmiddleware.RequestHandler
	var responseHandlers []awsmiddleware.ResponseHandler
	var statsProviders []agent.StatsProvider

	if !statusCodeEnabled && !agentStatsEnabled {
		return nil, nil
	}

	// Create and configure the StatusCodeHandler if enabled
	if statusCodeEnabled {
		log.Println("StatusCodeEnabled is true. Initializing StatusCodeHandler...")
		statusCodeFilter := agent.NewStatusCodeOperationsFilter()
		statusCodeStatsProvider := provider.GetStatsProvider(statusCodeFilter)
		statusCodeHandler := provider.NewStatusCodeHandler(statusCodeStatsProvider)

		// Add StatusCodeHandler to handlers
		requestHandlers = append(requestHandlers, statusCodeHandler)
		responseHandlers = append(responseHandlers, statusCodeHandler)
		statsProviders = append(statsProviders, statusCodeStatsProvider)
	}

	// Create and configure the clientStats handler if agentStatsEnabled
	if agentStatsEnabled {
		log.Println("AgentStatsEnabled is true. Initializing clientStats...")
		clientStats := client.NewHandler(agent.NewOperationsFilter())

		// Add clientStats and other providers to handlers and statsProviders
		statsProviders = append(statsProviders, clientStats, provider.GetProcessStats(), provider.GetFlagsStats())
		responseHandlers = append(responseHandlers, clientStats)
		requestHandlers = append(requestHandlers, clientStats)
	}

	// Create the primary stats handler with configured filters and providers
	filter := agent.NewOperationsFilter(cfg.Operations...)
	log.Println("Initializing primary stats handler...")
	stats := newStatsHandler(logger, filter, statsProviders)
	requestHandlers = append(requestHandlers, stats)

	// Apply usage flags configuration
	log.Println("Setting usage flags from configuration...")
	agent.UsageFlags().SetValues(cfg.UsageFlags)

	return requestHandlers, responseHandlers
}

type statsHandler struct {
	mu sync.Mutex

	logger    *zap.Logger
	filter    agent.OperationsFilter
	providers []agent.StatsProvider
}

func newStatsHandler(logger *zap.Logger, filter agent.OperationsFilter, providers []agent.StatsProvider) *statsHandler {
	sh := &statsHandler{
		logger:    logger,
		filter:    filter,
		providers: providers,
	}
	return sh
}

var _ awsmiddleware.RequestHandler = (*statsHandler)(nil)

func (sh *statsHandler) ID() string {
	return handlerID
}

func (sh *statsHandler) Position() awsmiddleware.HandlerPosition {
	return awsmiddleware.After
}

func (sh *statsHandler) HandleRequest(ctx context.Context, r *http.Request) {
	operation := awsmiddleware.GetOperationName(ctx)
	if !sh.filter.IsAllowed(operation) {
		return
	}
	header := sh.Header(operation)
	if header != "" {
		r.Header.Set(headerKeyAgentStats, header)
	}
}

func (sh *statsHandler) Header(operation string) string {
	stats := &agent.Stats{}
	for _, p := range sh.providers {
		stats.Merge(p.Stats(operation))
	}
	header, err := stats.Marshal()
	if err != nil {
		sh.logger.Warn("Failed to serialize agent stats", zap.Error(err))
	}
	return header
}
