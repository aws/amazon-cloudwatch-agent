// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package stats

import (
	"context"
	"fmt"
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

func NewHandlers(logger *zap.Logger, cfg agent.StatsConfig, statsB bool) ([]awsmiddleware.RequestHandler, []awsmiddleware.ResponseHandler) {
	if statsB {
		log.Println("Stats are enabled, creating handlers")

		// Create the operations filter
		filter := agent.NewOperationsFilter(cfg.Operations...)
		log.Println("Operations filter created, operations:", cfg.Operations)

		clientStats := client.NewHandler(filter)
		log.Println("Client stats handler created")

		statusCodeStats := provider.GetStatusCodeStats()
		log.Println("Status code stats handler retrieved")

		stats := newStatsHandler(logger, filter, []agent.StatsProvider{
			clientStats,
			provider.GetProcessStats(),
			provider.GetFlagsStats(),
			statusCodeStats,
		})
		log.Println("Stats handler created with providers")

		// Set usage flags
		agent.UsageFlags().SetValues(cfg.UsageFlags)

		// Return handlers
		log.Println("Returning request and response handlers, requestHandlerCount: 2, responseHandlerCount: 1")
		return []awsmiddleware.RequestHandler{stats, clientStats, statusCodeStats}, []awsmiddleware.ResponseHandler{statusCodeStats}
	} else {
		log.Println("Stats are disabled, creating only status code stats handler")

		statusCodeStats := provider.GetStatusCodeStats()
		log.Println("Status code stats handler retrieved")

		log.Println("Returning handlers, requestHandlerCount: 0, responseHandlerCount: 1")
		return []awsmiddleware.RequestHandler{statusCodeStats}, []awsmiddleware.ResponseHandler{statusCodeStats}
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
	log.Println("Generating header for operation:", operation)

	stats := &agent.Stats{}
	for _, p := range sh.providers {
		log.Println("Merging stats from provider:", fmt.Sprintf("%T", p))
		stats.Merge(p.Stats("PutMetricData"))
		stats.Merge(p.Stats("DescribeTags"))

	}

	log.Println("Stats after merging all providers:", stats)

	header, err := stats.Marshal()
	if err != nil {
		log.Println("Failed to serialize agent stats:", err)
		return ""
	}

	log.Println("Successfully generated header for operation:", operation)
	return header
}
