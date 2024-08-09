// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// Package cloudwatch provides a metric exporter for the OpenTelemetry collector.
package cloudwatch

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	stability = component.StabilityLevelAlpha
)

var (
	TypeStr, _ = component.NewType("awscloudwatch")
)

func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		TypeStr,
		createDefaultConfig,
		exporter.WithMetrics(createMetricsExporter, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Namespace:          "CWAgent",
		MaxDatumsPerCall:   defaultMaxDatumsPerCall,
		MaxValuesPerDatum:  defaultMaxValuesPerDatum,
		ForceFlushInterval: defaultForceFlushInterval,
		ResourceToTelemetrySettings: resourcetotelemetry.Settings{
			Enabled: true,
		},
	}
}

func createMetricsExporter(
	ctx context.Context,
	settings exporter.CreateSettings,
	config component.Config,
) (exporter.Metrics, error) {
	cw := &CloudWatch{
		config: config.(*Config),
		logger: settings.Logger,
	}
	exp, err := exporterhelper.NewMetricsExporter(
		ctx,
		settings,
		config,
		cw.ConsumeMetrics,
		exporterhelper.WithStart(cw.Start),
		exporterhelper.WithShutdown(cw.Shutdown),
	)
	if err != nil {
		return nil, err
	}
	return resourcetotelemetry.WrapMetricsExporter(
		config.(*Config).ResourceToTelemetrySettings, exp), nil
}
