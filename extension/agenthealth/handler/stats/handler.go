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

func NewHandlers(logger *zap.Logger, cfg agent.StatsConfig, statusCodeEnabled bool, agentStatsEnabled bool) ([]awsmiddleware.RequestHandler, []awsmiddleware.ResponseHandler) {
	log.Println("creating New handler")

	var requestHandlers []awsmiddleware.RequestHandler
	var responseHandlers []awsmiddleware.ResponseHandler
	var statsProviders []agent.StatsProvider

	if !statusCodeEnabled && !agentStatsEnabled {
		return nil, nil
	}

	statusCodeFilter := agent.NewStatusCodeOperationsFilter()
	statusCodeStats := provider.GetStatusCodeStats(statusCodeFilter)
	if statusCodeEnabled {
		log.Println("StatusCode is enabled!")
		requestHandlers = append(requestHandlers, statusCodeStats)
		responseHandlers = append(responseHandlers, statusCodeStats)
		statsProviders = append(statsProviders, statusCodeStats)
	}

	if agentStatsEnabled {
		clientStats := client.NewHandler(agent.NewOperationsFilter())
		statsProviders = append(statsProviders, clientStats, provider.GetProcessStats(), provider.GetFlagsStats())
		responseHandlers = append(responseHandlers, clientStats)
		requestHandlers = append(requestHandlers, clientStats)

	}
	filter := agent.NewStatusCodeAndOtherOperationsFilter(cfg.Operations)
	stats := newStatsHandler(logger, filter, statsProviders)
	requestHandlers = append(requestHandlers, stats)

	agent.UsageFlags().SetValues(cfg.UsageFlags)
	log.Println("Request Handlers:")
	log.Println(requestHandlers)
	log.Println("Response Handlers:")
	log.Println(responseHandlers)
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
	log.Println("Handling request for operation:", operation)

	if !sh.filter.IsAllowed(operation) {
		log.Println("Operation not allowed:", operation)
		return
	}

	log.Println("Generating header for operation:", operation)
	header := sh.Header(operation)

	log.Println("This is the header", header)
	if header != "" {
		log.Println("Setting header for operation:", operation)
		r.Header.Set(headerKeyAgentStats, header)
		log.Println("Header set successfully for operation:", operation)
	} else {
		log.Println("No header generated for operation:", operation)
	}
}

func (sh *statsHandler) Header(operation string) string {
	log.Println("Generating header for operation:", operation)

	stats := &agent.Stats{}
	for _, p := range sh.providers {
		log.Println("Merging stats from provider:", fmt.Sprintf("%T", p))
		stats.Merge(p.Stats(operation))

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
