// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2tagger

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const (
	TypeStr   = "ec2tagger"
	stability = component.StabilityLevelStable
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

func createDefaultConfig() config.Processor {
	return &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(TypeStr)),
	}
}

func NewFactory() component.ProcessorFactory {
	return component.NewProcessorFactory(
		TypeStr,
		createDefaultConfig,
		component.WithMetricsProcessor(createMetricsProcessor, stability))
}

func createMetricsProcessor(
	ctx context.Context,
	set component.ProcessorCreateSettings,
	cfg config.Processor,
	nextConsumer consumer.Metrics,
) (component.MetricsProcessor, error) {
	processorConfig, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("configuration parsing error")
	}

	metricsProcessor := newTagger(processorConfig, set.Logger)

	return processorhelper.NewMetricsProcessor(ctx, set, cfg, nextConsumer,
		metricsProcessor.processMetrics,
		processorhelper.WithCapabilities(processorCapabilities),
		processorhelper.WithStart(metricsProcessor.Start),
		processorhelper.WithShutdown(metricsProcessor.Shutdown))
}
