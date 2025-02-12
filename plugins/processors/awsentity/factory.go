// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsentity

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const (
	stability = component.StabilityLevelBeta
)

var (
	TypeStr, _            = component.NewType("awsentity")
	processorCapabilities = consumer.Capabilities{MutatesData: false}
)

func NewFactory() processor.Factory {
	return processor.NewFactory(
		TypeStr,
		createDefaultConfig,
		processor.WithMetrics(createMetricsProcessor, stability))
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	processorConfig, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("configuration parsing error")
	}
	metricsProcessor := newAwsEntityProcessor(processorConfig, set.Logger)

	return processorhelper.NewMetrics(
		ctx,
		set,
		cfg,
		nextConsumer,
		metricsProcessor.processMetrics,
		processorhelper.WithCapabilities(processorCapabilities))
}
