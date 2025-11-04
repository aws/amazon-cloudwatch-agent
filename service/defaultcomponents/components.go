// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package defaultcomponents

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/awsproxy"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/sigv4authextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatorateprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricsgenerationprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightskueuereceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcplogreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/udplogreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/debugexporter"
	"go.opentelemetry.io/collector/exporter/nopexporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/zpagesextension"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/processor/memorylimiterprocessor"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/nopreceiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/extension/entitystore"
	"github.com/aws/amazon-cloudwatch-agent/extension/k8smetadata"
	"github.com/aws/amazon-cloudwatch-agent/extension/server"
	"github.com/aws/amazon-cloudwatch-agent/plugins/outputs/cloudwatch"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsentity"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/ec2tagger"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/gpuattributes"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/kueueattributes"
	"github.com/aws/amazon-cloudwatch-agent/processor/rollupprocessor"
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsnvmereceiver"
)

func Factories() (otelcol.Factories, error) {
	var factories otelcol.Factories
	var err error

	if factories.Receivers, err = otelcol.MakeFactoryMap[receiver.Factory](
		awscontainerinsightreceiver.NewFactory(),
		awscontainerinsightskueuereceiver.NewFactory(),
		awsecscontainermetricsreceiver.NewFactory(),
		awsnvmereceiver.NewFactory(),
		awsxrayreceiver.NewFactory(),
		filelogreceiver.NewFactory(),
		jaegerreceiver.NewFactory(),
		jmxreceiver.NewFactory(),
		kafkareceiver.NewFactory(),
		nopreceiver.NewFactory(),
		otlpreceiver.NewFactory(),
		prometheusreceiver.NewFactory(),
		statsdreceiver.NewFactory(),
		tcplogreceiver.NewFactory(),
		udplogreceiver.NewFactory(),
		zipkinreceiver.NewFactory(),
	); err != nil {
		return otelcol.Factories{}, err
	}

	if factories.Processors, err = otelcol.MakeFactoryMap[processor.Factory](
		attributesprocessor.NewFactory(),
		awsapplicationsignals.NewFactory(),
		awsentity.NewFactory(),
		batchprocessor.NewFactory(),
		cumulativetodeltaprocessor.NewFactory(),
		deltatocumulativeprocessor.NewFactory(),
		deltatorateprocessor.NewFactory(),
		ec2tagger.NewFactory(),
		filterprocessor.NewFactory(),
		gpuattributes.NewFactory(),
		kueueattributes.NewFactory(),
		groupbytraceprocessor.NewFactory(),
		groupbyattrsprocessor.NewFactory(),
		k8sattributesprocessor.NewFactory(),
		memorylimiterprocessor.NewFactory(),
		metricsgenerationprocessor.NewFactory(),
		metricstransformprocessor.NewFactory(),
		probabilisticsamplerprocessor.NewFactory(),
		resourceprocessor.NewFactory(),
		resourcedetectionprocessor.NewFactory(),
		rollupprocessor.NewFactory(),
		spanprocessor.NewFactory(),
		tailsamplingprocessor.NewFactory(),
		transformprocessor.NewFactory(),
	); err != nil {
		return otelcol.Factories{}, err
	}

	if factories.Exporters, err = otelcol.MakeFactoryMap[exporter.Factory](
		awscloudwatchlogsexporter.NewFactory(),
		awsemfexporter.NewFactory(),
		awsxrayexporter.NewFactory(),
		cloudwatch.NewFactory(),
		debugexporter.NewFactory(),
		nopexporter.NewFactory(),
		prometheusremotewriteexporter.NewFactory(),
	); err != nil {
		return otelcol.Factories{}, err
	}

	if factories.Extensions, err = otelcol.MakeFactoryMap[extension.Factory](
		agenthealth.NewFactory(),
		awsproxy.NewFactory(),
		entitystore.NewFactory(),
		k8smetadata.NewFactory(),
		server.NewFactory(),
		ecsobserver.NewFactory(),
		filestorage.NewFactory(),
		healthcheckextension.NewFactory(),
		pprofextension.NewFactory(),
		sigv4authextension.NewFactory(),
		zpagesextension.NewFactory(),
	); err != nil {
		return otelcol.Factories{}, err
	}

	return factories, nil
}
