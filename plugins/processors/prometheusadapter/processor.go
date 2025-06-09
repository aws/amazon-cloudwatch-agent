// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheusadapter

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type prometheusAdapterProcessor struct {
	*Config
	logger *zap.Logger
}

func newPrometheusAdapterProcessor(config *Config, logger *zap.Logger) *prometheusAdapterProcessor {
	d := &prometheusAdapterProcessor{
		Config: config,
		logger: logger,
	}
	return d
}

func (d *prometheusAdapterProcessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	return md, nil
}
