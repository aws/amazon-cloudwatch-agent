// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package stats

import (
	"context"
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

func NewHandlers(logger *zap.Logger, cfg agent.StatsConfig, statsB bool) ([]awsmiddleware.RequestHandler, []awsmiddleware.ResponseHandler) {
	if statsB {
		logger.Debug("Stats are enabled, creating handlers")

		// Create the operations filter
		filter := agent.NewOperationsFilter(cfg.Operations...)
		logger.Debug("Operations filter created", zap.Strings("operations", cfg.Operations))

		// Create client stats handler
		clientStats := client.NewHandler(filter)
		logger.Debug("Client stats handler created")

		// Get status code stats
		statusCodeStats := provider.GetStatusCodeStats()
		logger.Debug("Status code stats handler retrieved")

		// Create stats handler
		stats := newStatsHandler(logger, filter, []agent.StatsProvider{
			clientStats,
			provider.GetProcessStats(),
			provider.GetFlagsStats(),
			statusCodeStats,
		})
		logger.Debug("Stats handler created with providers")

		// Set usage flags
		agent.UsageFlags().SetValues(cfg.UsageFlags)

		// Return handlers
		logger.Debug("Returning request and response handlers",
			zap.Int("requestHandlerCount", 2),
			zap.Int("responseHandlerCount", 1),
		)
		return []awsmiddleware.RequestHandler{stats, clientStats}, []awsmiddleware.ResponseHandler{statusCodeStats}
	} else {
		logger.Debug("Stats are disabled, creating only status code stats handler")

		// Get status code stats
		statusCodeStats := provider.GetStatusCodeStats()
		logger.Debug("Status code stats handler retrieved")

		// Return empty request handlers and response handlers with status code stats
		logger.Debug("Returning handlers",
			zap.Int("requestHandlerCount", 0),
			zap.Int("responseHandlerCount", 1),
		)
		return []awsmiddleware.RequestHandler{}, []awsmiddleware.ResponseHandler{statusCodeStats}
	}
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
	//if !sh.filter.IsAllowed(operation) {
	//	return
	//}
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
