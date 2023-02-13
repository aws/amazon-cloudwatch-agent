// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// Package cloudwatch provides a metric exporter for the OpenTelemetry collector.
// todo: Once the private and public repositories are merged it would be good
// to move this package to .../exporter/awscloudwatch and rename it.
package cloudwatch

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	TypeStr   component.Type = "awscloudwatch"
	stability                = component.StabilityLevelAlpha
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
		ExporterSettings:   config.NewExporterSettings(component.NewID(TypeStr)),
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
	exp := &CloudWatch{
		config: config.(*Config),
	}
	exporter, err := exporterhelper.NewMetricsExporter(
		ctx,
		settings,
		config,
		exp.ConsumeMetrics,
		exporterhelper.WithStart(exp.Start),
		exporterhelper.WithShutdown(exp.Shutdown),
	)
	if err != nil {
		return nil, err
	}
	return resourcetotelemetry.WrapMetricsExporter(
		config.(*Config).ResourceToTelemetrySettings, exporter), nil
}
