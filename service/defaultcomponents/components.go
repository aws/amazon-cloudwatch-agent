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
	"go.opentelemetry.io/collector/component"
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
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsebsnvmereceiver"
)

func Factories() (otelcol.Factories, error) {
	factories := otelcol.Factories{}

	// Create a map for receivers
	receivers := map[component.Type]receiver.Factory{
		awscontainerinsightreceiver.NewFactory().Type():       awscontainerinsightreceiver.NewFactory(),
		awscontainerinsightskueuereceiver.NewFactory().Type(): awscontainerinsightskueuereceiver.NewFactory(),
		awsecscontainermetricsreceiver.NewFactory().Type():    awsecscontainermetricsreceiver.NewFactory(),
		awsebsnvmereceiver.NewFactory().Type():                awsebsnvmereceiver.NewFactory(),
		awsxrayreceiver.NewFactory().Type():                   awsxrayreceiver.NewFactory(),
		filelogreceiver.NewFactory().Type():                   filelogreceiver.NewFactory(),
		jaegerreceiver.NewFactory().Type():                    jaegerreceiver.NewFactory(),
		jmxreceiver.NewFactory().Type():                       jmxreceiver.NewFactory(),
		kafkareceiver.NewFactory().Type():                     kafkareceiver.NewFactory(),
		nopreceiver.NewFactory().Type():                       nopreceiver.NewFactory(),
		otlpreceiver.NewFactory().Type():                      otlpreceiver.NewFactory(),
		prometheusreceiver.NewFactory().Type():                prometheusreceiver.NewFactory(),
		statsdreceiver.NewFactory().Type():                    statsdreceiver.NewFactory(),
		tcplogreceiver.NewFactory().Type():                    tcplogreceiver.NewFactory(),
		udplogreceiver.NewFactory().Type():                    udplogreceiver.NewFactory(),
		zipkinreceiver.NewFactory().Type():                    zipkinreceiver.NewFactory(),
	}
	factories.Receivers = receivers

	// Create a map for processors
	processors := map[component.Type]processor.Factory{
		attributesprocessor.NewFactory().Type():           attributesprocessor.NewFactory(),
		awsapplicationsignals.NewFactory().Type():         awsapplicationsignals.NewFactory(),
		awsentity.NewFactory().Type():                     awsentity.NewFactory(),
		batchprocessor.NewFactory().Type():                batchprocessor.NewFactory(),
		cumulativetodeltaprocessor.NewFactory().Type():    cumulativetodeltaprocessor.NewFactory(),
		deltatocumulativeprocessor.NewFactory().Type():    deltatocumulativeprocessor.NewFactory(),
		deltatorateprocessor.NewFactory().Type():          deltatorateprocessor.NewFactory(),
		ec2tagger.NewFactory().Type():                     ec2tagger.NewFactory(),
		filterprocessor.NewFactory().Type():               filterprocessor.NewFactory(),
		gpuattributes.NewFactory().Type():                 gpuattributes.NewFactory(),
		kueueattributes.NewFactory().Type():               kueueattributes.NewFactory(),
		groupbytraceprocessor.NewFactory().Type():         groupbytraceprocessor.NewFactory(),
		k8sattributesprocessor.NewFactory().Type():        k8sattributesprocessor.NewFactory(),
		memorylimiterprocessor.NewFactory().Type():        memorylimiterprocessor.NewFactory(),
		metricsgenerationprocessor.NewFactory().Type():    metricsgenerationprocessor.NewFactory(),
		metricstransformprocessor.NewFactory().Type():     metricstransformprocessor.NewFactory(),
		probabilisticsamplerprocessor.NewFactory().Type(): probabilisticsamplerprocessor.NewFactory(),
		resourceprocessor.NewFactory().Type():             resourceprocessor.NewFactory(),
		resourcedetectionprocessor.NewFactory().Type():    resourcedetectionprocessor.NewFactory(),
		rollupprocessor.NewFactory().Type():               rollupprocessor.NewFactory(),
		spanprocessor.NewFactory().Type():                 spanprocessor.NewFactory(),
		tailsamplingprocessor.NewFactory().Type():         tailsamplingprocessor.NewFactory(),
		transformprocessor.NewFactory().Type():            transformprocessor.NewFactory(),
	}
	factories.Processors = processors

	// Create a map for exporters
	exporters := map[component.Type]exporter.Factory{
		awscloudwatchlogsexporter.NewFactory().Type():     awscloudwatchlogsexporter.NewFactory(),
		awsemfexporter.NewFactory().Type():                awsemfexporter.NewFactory(),
		awsxrayexporter.NewFactory().Type():               awsxrayexporter.NewFactory(),
		cloudwatch.NewFactory().Type():                    cloudwatch.NewFactory(),
		debugexporter.NewFactory().Type():                 debugexporter.NewFactory(),
		nopexporter.NewFactory().Type():                   nopexporter.NewFactory(),
		prometheusremotewriteexporter.NewFactory().Type(): prometheusremotewriteexporter.NewFactory(),
	}
	factories.Exporters = exporters

	// Create a map for extensions
	extensions := map[component.Type]extension.Factory{
		agenthealth.NewFactory().Type():          agenthealth.NewFactory(),
		awsproxy.NewFactory().Type():             awsproxy.NewFactory(),
		entitystore.NewFactory().Type():          entitystore.NewFactory(),
		k8smetadata.NewFactory().Type():          k8smetadata.NewFactory(),
		server.NewFactory().Type():               server.NewFactory(),
		ecsobserver.NewFactory().Type():          ecsobserver.NewFactory(),
		filestorage.NewFactory().Type():          filestorage.NewFactory(),
		healthcheckextension.NewFactory().Type(): healthcheckextension.NewFactory(),
		pprofextension.NewFactory().Type():       pprofextension.NewFactory(),
		sigv4authextension.NewFactory().Type():   sigv4authextension.NewFactory(),
		zpagesextension.NewFactory().Type():      zpagesextension.NewFactory(),
	}
	factories.Extensions = extensions

	return factories, nil
}
