// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agenthealth

import (
	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/useragent"
)

type agentHealth struct {
	logger *zap.Logger
	cfg    *Config
	component.StartFunc
	component.ShutdownFunc
}

var _ awsmiddleware.Extension = (*agentHealth)(nil)

func (ah *agentHealth) Handlers() ([]awsmiddleware.RequestHandler, []awsmiddleware.ResponseHandler) {
	var responseHandlers []awsmiddleware.ResponseHandler
	requestHandlers := []awsmiddleware.RequestHandler{useragent.NewHandler(ah.cfg.IsUsageDataEnabled)}

	if !ah.cfg.IsUsageDataEnabled {
		ah.logger.Debug("Usage data is disabled, skipping stats handlers")
		return requestHandlers, responseHandlers
	}

	if ah.cfg == nil {
		return nil, nil
	}
	statusCodeEnabled := ah.cfg.IsStatusCodeEnabled

	var statsResponseHandlers []awsmiddleware.ResponseHandler
	var statsRequestHandlers []awsmiddleware.RequestHandler

	if ah.cfg.Stats != nil {
		statsRequestHandlers, statsResponseHandlers = stats.NewHandlers(ah.logger, *ah.cfg.Stats, statusCodeEnabled, true)
	} else {
		statsRequestHandlers, statsResponseHandlers = stats.NewHandlers(ah.logger, agent.StatsConfig{}, statusCodeEnabled, false)
	}

	requestHandlers = append(requestHandlers, statsRequestHandlers...)
	responseHandlers = append(responseHandlers, statsResponseHandlers...)

	return requestHandlers, responseHandlers
}

func NewAgentHealth(logger *zap.Logger, cfg *Config) awsmiddleware.Extension {
	return &agentHealth{logger: logger, cfg: cfg}
}
