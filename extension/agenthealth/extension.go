// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agenthealth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.opentelemetry.io/collector/extension/extensioncapabilities"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/useragent"
)

type agentHealth struct {
	logger *zap.Logger
	cfg    *Config
	host   component.Host
	component.ShutdownFunc
}

var _ awsmiddleware.Extension = (*agentHealth)(nil)
var _ extensionauth.HTTPClient = (*agentHealth)(nil)
var _ extensioncapabilities.Dependent = (*agentHealth)(nil)

func (ah *agentHealth) Handlers() ([]awsmiddleware.RequestHandler, []awsmiddleware.ResponseHandler) {
	var responseHandlers []awsmiddleware.ResponseHandler
	requestHandlers := []awsmiddleware.RequestHandler{useragent.NewHandler(ah.cfg.IsUsageDataEnabled)}

	if !ah.cfg.IsUsageDataEnabled {
		ah.logger.Debug("Usage data is disabled, skipping stats handlers")
		return requestHandlers, responseHandlers
	}

	useragent.Get().AddFeatureFlags(ah.cfg.UsageMetadata...)
	statusCodeEnabled := ah.cfg.IsStatusCodeEnabled

	var statsResponseHandlers []awsmiddleware.ResponseHandler
	var statsRequestHandlers []awsmiddleware.RequestHandler
	var statsConfig agent.StatsConfig
	var agentStatsEnabled bool

	if ah.cfg.Stats != nil {
		statsConfig = *ah.cfg.Stats
		agentStatsEnabled = true
	} else {
		agentStatsEnabled = false
	}

	statsRequestHandlers, statsResponseHandlers = stats.NewHandlers(ah.logger, statsConfig, statusCodeEnabled, agentStatsEnabled)

	requestHandlers = append(requestHandlers, statsRequestHandlers...)
	responseHandlers = append(responseHandlers, statsResponseHandlers...)

	return requestHandlers, responseHandlers
}

func (ah *agentHealth) Start(_ context.Context, host component.Host) error {
	ah.host = host
	return nil
}

func (ah *agentHealth) Dependencies() []component.ID {
	if ah.cfg.AdditionalAuth == nil {
		return nil
	}
	return []component.ID{*ah.cfg.AdditionalAuth}
}

func (ah *agentHealth) getAdditionalAuthExtension() (component.Component, error) {
	if ah.cfg.AdditionalAuth == nil || ah.host == nil {
		return nil, nil
	}
	ext := ah.host.GetExtensions()[*ah.cfg.AdditionalAuth]
	if ext == nil {
		return nil, fmt.Errorf("auth extension %v not found", ah.cfg.AdditionalAuth)
	}
	return ext, nil
}

func (ah *agentHealth) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	ext, err := ah.getAdditionalAuthExtension()
	if err != nil {
		return nil, err
	}
	if ext != nil {
		httpClient, ok := ext.(extensionauth.HTTPClient)
		if !ok {
			return nil, fmt.Errorf("auth extension %v does not implement extensionauth.HTTPClient", ah.cfg.AdditionalAuth)
		}
		base, err = httpClient.RoundTripper(base)
		if err != nil {
			return nil, fmt.Errorf("failed to get RoundTripper from %v: %w", ah.cfg.AdditionalAuth, err)
		}
	}
	requestHandlers, responseHandlers := ah.Handlers()
	return &roundTripper{
		base:             base,
		requestHandlers:  requestHandlers,
		responseHandlers: responseHandlers,
	}, nil
}

func NewAgentHealth(logger *zap.Logger, cfg *Config) awsmiddleware.Extension {
	return &agentHealth{logger: logger, cfg: cfg}
}
