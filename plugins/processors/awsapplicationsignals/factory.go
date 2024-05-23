// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsapplicationsignals

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"

	appsignalsconfig "github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/config"
)

const (
	// The stability level of the processor.
	stability = component.StabilityLevelBeta
)

var (
	// The value of "type" key in configuration.
	typeStr, _           = component.NewType("awsapplicationsignals")
	consumerCapabilities = consumer.Capabilities{MutatesData: true}
)

// NewFactory returns a new factory for the aws attributes processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		typeStr,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, stability),
		processor.WithMetrics(createMetricsProcessor, stability),
	)
}

func createDefaultConfig() component.Config {
	return &appsignalsconfig.Config{
		Resolvers: []appsignalsconfig.Resolver{},
	}
}

func createTracesProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	next consumer.Traces,
) (processor.Traces, error) {
	ap, err := createProcessor(set, cfg)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewTracesProcessor(
		ctx,
		set,
		cfg,
		next,
		ap.processTraces,
		processorhelper.WithCapabilities(consumerCapabilities),
		processorhelper.WithStart(ap.StartTraces),
		processorhelper.WithShutdown(ap.Shutdown))
}

func createMetricsProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextMetricsConsumer consumer.Metrics,
) (processor.Metrics, error) {
	ap, err := createProcessor(set, cfg)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewMetricsProcessor(
		ctx,
		set,
		cfg,
		nextMetricsConsumer,
		ap.processMetrics,
		processorhelper.WithCapabilities(consumerCapabilities),
		processorhelper.WithStart(ap.StartMetrics),
		processorhelper.WithShutdown(ap.Shutdown))
}

func createProcessor(
	params processor.CreateSettings,
	cfg component.Config,
) (*awsapplicationsignalsprocessor, error) {
	pCfg, ok := cfg.(*appsignalsconfig.Config)
	if !ok {
		return nil, errors.New("could not initialize awsapplicationsignalsprocessor")
	}
	ap := &awsapplicationsignalsprocessor{logger: params.Logger, config: pCfg}

	return ap, nil
}
