// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package defaultcomponents

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcplogreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/udplogreceiver"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/loggingexporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"

	"github.com/aws/amazon-cloudwatch-agent/plugins/outputs/cloudwatch"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/ec2tagger"
)

func Factories() (otelcol.Factories, error) {
	var factories otelcol.Factories
	var err error

	if factories.Receivers, err = receiver.MakeFactoryMap(
		awscontainerinsightreceiver.NewFactory(),
		awsxrayreceiver.NewFactory(),
		otlpreceiver.NewFactory(),
		tcplogreceiver.NewFactory(),
		udplogreceiver.NewFactory(),
	); err != nil {
		return otelcol.Factories{}, err
	}

	if factories.Processors, err = processor.MakeFactoryMap(
		batchprocessor.NewFactory(),
		cumulativetodeltaprocessor.NewFactory(),
		ec2tagger.NewFactory(),
		transformprocessor.NewFactory(),
	); err != nil {
		return otelcol.Factories{}, err
	}

	if factories.Exporters, err = exporter.MakeFactoryMap(
		awscloudwatchlogsexporter.NewFactory(),
		awsemfexporter.NewFactory(),
		awsxrayexporter.NewFactory(),
		cloudwatch.NewFactory(),
		loggingexporter.NewFactory(),
	); err != nil {
		return otelcol.Factories{}, err
	}

	if factories.Extensions, err = extension.MakeFactoryMap(); err != nil {
		return otelcol.Factories{}, err
	}

	return factories, nil
}
