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

func NewHandlers(logger *zap.Logger, cfg agent.StatsConfig) ([]awsmiddleware.RequestHandler, []awsmiddleware.ResponseHandler) {
	filter := agent.NewOperationsFilter(cfg.Operations...)
	clientStats := client.NewHandler(filter)
	stats := newStatsHandler(logger, filter, []agent.StatsProvider{clientStats, provider.GetProcessStats(), provider.GetFlagsStats()})
	agent.UsageFlags().SetValues(cfg.UsageFlags)
	return []awsmiddleware.RequestHandler{stats, clientStats}, []awsmiddleware.ResponseHandler{clientStats}
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
