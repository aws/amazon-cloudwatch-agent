// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package stats

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/client"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/provider"
)

const (
	handlerID           = "cloudwatchagent.StatsHandler"
	headerKeyAgentStats = "X-Amz-Agent-Stats"
)

func NewHandlers(logger *zap.Logger, cfg client.StatsConfig) ([]awsmiddleware.RequestHandler, []awsmiddleware.ResponseHandler) {
	clientStats := client.NewHandler(cfg)
	stats := newStatsHandler(logger, []agent.StatsProvider{clientStats, provider.GetProcessStats(), provider.GetFlagsStats()})
	return []awsmiddleware.RequestHandler{clientStats, stats}, []awsmiddleware.ResponseHandler{clientStats, stats}
}

type statsHandler struct {
	mu        sync.Mutex
	logger    *zap.Logger
	providers []agent.StatsProvider

	headers map[string]string
}

func newStatsHandler(logger *zap.Logger, providers []agent.StatsProvider) *statsHandler {
	sh := &statsHandler{
		logger:    logger,
		providers: providers,
		headers:   make(map[string]string),
	}
	return sh
}

var _ awsmiddleware.RequestHandler = (*statsHandler)(nil)
var _ awsmiddleware.ResponseHandler = (*statsHandler)(nil)

func (sh *statsHandler) ID() string {
	return handlerID
}

func (sh *statsHandler) Position() awsmiddleware.HandlerPosition {
	return awsmiddleware.After
}

func (sh *statsHandler) HandleRequest(ctx context.Context, r *http.Request) {
	header := sh.Header(awsmiddleware.GetOperationName(ctx))
	if header != "" {
		r.Header.Set(headerKeyAgentStats, header)
	}
}

func (sh *statsHandler) HandleResponse(ctx context.Context, _ *http.Response) {
	go sh.refreshHeader(awsmiddleware.GetOperationName(ctx))
}

func (sh *statsHandler) Header(operation string) string {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	return sh.headers[operation]
}

func (sh *statsHandler) refreshHeader(operation string) {
	stats := agent.Stats{}
	for _, p := range sh.providers {
		stats.Merge(p.Stats(operation))
	}
	sh.mu.Lock()
	defer sh.mu.Unlock()
	sh.headers[operation] = sh.getHeader(stats)
}

func (sh *statsHandler) getHeader(stats agent.Stats) string {
	raw, err := json.Marshal(stats)
	if err != nil {
		sh.logger.Warn("Failed to serialize agent stats", zap.Error(err))
		return ""
	}
	content := strings.TrimPrefix(string(raw), "{")
	return strings.TrimSuffix(content, "}")
}
