// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package opentelemetry

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/connector/forward"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/connector/spanmetrics"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/otlphttp"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/sigv4auth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/batchprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/k8sattributesprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/resourcedetection"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/transformprocessor"
)

var otelMetricsKeys = []string{
	common.ConfigKey(common.OpenTelemetryKey, common.CollectKey, common.OtlpKey),
	common.ConfigKey(common.OpenTelemetryKey, common.CollectKey, common.HostMetricsKey),
	common.ConfigKey(common.OpenTelemetryKey, common.CollectKey, common.PrometheusKey),
	common.DatabaseInsightsConfigKey,
	common.ConfigKey(common.OpenTelemetryKey, common.CollectKey, common.OtelContainerInsightsKey),
}

type baseMetricsTranslator struct{}

var _ common.PipelineTranslator = (*baseMetricsTranslator)(nil)

func NewBaseMetricsTranslator() common.PipelineTranslator {
	return &baseMetricsTranslator{}
}

func (t *baseMetricsTranslator) ID() pipeline.ID {
	return pipeline.NewIDWithName(pipeline.SignalMetrics, common.OpenTelemetryKey)
}

func (t *baseMetricsTranslator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if err := common.ValidateAnySet(conf, t.ID(), otelMetricsKeys); err != nil {
		return nil, err
	}

	region := agent.Global_Config.Region
	if region == "" {
		return nil, fmt.Errorf("region is required for %s pipeline", common.OpenTelemetryKey)
	}
	metricsEndpoint := common.ServiceEndpoint("monitoring", region, "/v1/metrics")
	sigv4Ext := sigv4auth.NewTranslatorWithService("monitoring")
	agentHealthExt := agenthealth.NewTranslator(agenthealth.OtelMetricsName, []string{"*"}, agenthealth.WithAdditionalAuth(sigv4Ext.ID()))

	fwdConnector := forward.NewTranslator(common.OpenTelemetryKey)

	processors := common.NewTranslatorMap[component.Config, component.ID](resourcedetection.NewTranslator(resourcedetection.WithName(common.OpenTelemetryKey)))
	if context.CurrentContext().KubernetesMode() != "" {
		processors.Set(k8sattributesprocessor.NewTranslator(common.OpenTelemetryKey))
	}
	// Apply root-level cluster name if set
	clusterName := common.GetClusterName(conf, common.OtelClusterNameKey)
	if clusterName != "" {
		if err := common.ValidateClusterName(clusterName); err != nil {
			return nil, err
		}
		stmt := fmt.Sprintf(`set(resource.attributes["k8s.cluster.name"], "%s")`, clusterName)
		processors.Set(transformprocessor.NewTranslatorWithName("set_cluster_name",
			transformprocessor.WithMetricResourceStatements([]string{stmt}),
			transformprocessor.WithLogResourceStatements([]string{stmt}),
			transformprocessor.WithTraceResourceStatements([]string{stmt}),
		))
	}
	processors.Set(transformprocessor.NewTranslatorWithName(common.Identity))
	processors.Set(batchprocessor.NewTranslator(common.WithName("opentelemetry_metrics"), batchprocessor.WithSendBatchSize(common.MaxMetricsPerRequest), batchprocessor.WithSendBatchMaxSize(common.MaxMetricsPerRequest), batchprocessor.WithTimeout(common.BatchTimeout)))

	receivers := common.NewTranslatorMap[component.Config, component.ID](fwdConnector)
	connectors := common.NewTranslatorMap[component.Config, component.ID](fwdConnector)

	if common.GetOrDefaultBool(conf, common.OtelSpanMetricsEnabledKey, false) {
		sm := spanmetrics.NewTranslator(common.OpenTelemetryKey)
		receivers.Set(sm)
		connectors.Set(sm)
	}

	return &common.ComponentTranslators{
		Receivers:  receivers,
		Processors: processors,
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](otlphttp.NewTranslatorWithName("metrics", otlphttp.EndpointConfig{MetricsEndpoint: metricsEndpoint}, otlphttp.WithAuthenticator(agentHealthExt.ID()))),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](sigv4Ext, agentHealthExt),
		Connectors: connectors,
	}, nil
}
