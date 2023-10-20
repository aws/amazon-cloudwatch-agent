// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agenthealth

import (
	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

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
	var responsesHandlers []awsmiddleware.ResponseHandler
	requestsHandlers := []awsmiddleware.RequestHandler{useragent.NewHandler(ah.cfg.IsUsageDataEnabled)}
	return requestsHandlers, responsesHandlers
}

func newAgentHealth(logger *zap.Logger, cfg *Config) (*agentHealth, error) {
	return &agentHealth{logger: logger, cfg: cfg}, nil
}
