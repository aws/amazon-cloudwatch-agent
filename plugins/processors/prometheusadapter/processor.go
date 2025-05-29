// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheusadapter

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type prometheusAdapaterProcessor struct {
	*Config
	logger *zap.Logger
}

func newPrometheusAdapterProcessor(config *Config, logger *zap.Logger) *prometheusAdapaterProcessor {
	d := &prometheusAdapaterProcessor{
		Config: config,
		logger: logger,
	}
	return d
}

func (d *prometheusAdapaterProcessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	return md, nil
}
