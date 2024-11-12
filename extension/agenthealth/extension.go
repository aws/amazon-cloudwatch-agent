// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agenthealth

import (
	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats"
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
	// Initialize handlers
	var responseHandlers []awsmiddleware.ResponseHandler
	requestHandlers := []awsmiddleware.RequestHandler{useragent.NewHandler(ah.cfg.IsUsageDataEnabled)}

	// Log the initialization
	ah.logger.Debug("Initialized default request handler with user agent handler")

	if ah.cfg.IsUsageDataEnabled {
		// Log usage data configuration
		ah.logger.Debug("Usage data is enabled, creating stats handlers")

		// Create stats handlers
		req, res := stats.NewHandlers(ah.logger, ah.cfg.Stats, true)

		// Add stats handlers
		ah.logger.Debug("Appending request handlers", zap.Int("count", len(req)))
		requestHandlers = append(requestHandlers, req...)

		ah.logger.Debug("Appending response handlers", zap.Int("count", len(res)))
		responseHandlers = append(responseHandlers, res...)
	} else {
		// Log usage data configuration is disabled
		ah.logger.Debug("Usage data is disabled, skipping stats handlers")
	}

	// Log final handlers
	ah.logger.Debug("Handlers created",
		zap.Int("requestHandlersCount", len(requestHandlers)),
		zap.Int("responseHandlersCount", len(responseHandlers)),
	)

	return requestHandlers, responseHandlers
}

func NewAgentHealth(logger *zap.Logger, cfg *Config) awsmiddleware.Extension {
	return &agentHealth{logger: logger, cfg: cfg}
}
