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

type baseTracesTranslator struct{}

var _ common.PipelineTranslator = (*baseTracesTranslator)(nil)

func NewBaseTracesTranslator() common.PipelineTranslator {
	return &baseTracesTranslator{}
}

func (t *baseTracesTranslator) ID() pipeline.ID {
	return pipeline.NewIDWithName(pipeline.SignalTraces, common.OpenTelemetryKey)
}

// Translate creates the shared traces export pipeline. It activates when a
// traces source (otlp) is configured under opentelemetry.collect.
func (t *baseTracesTranslator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	otlpTracesKey := common.ConfigKey(common.OpenTelemetryKey, common.CollectKey, common.OtlpKey)
	if conf == nil || !conf.IsSet(otlpTracesKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: otlpTracesKey}
	}

	region := agent.Global_Config.Region
	if region == "" {
		return nil, fmt.Errorf("region is required for %s traces pipeline", common.OpenTelemetryKey)
	}
	tracesEndpoint := common.ServiceEndpoint("xray", region, "/v1/traces")
	sigv4Ext := sigv4auth.NewTranslatorWithService("xray")
	agentHealthExt := agenthealth.NewTranslator(agenthealth.OtelTracesName, []string{"*"}, agenthealth.WithAdditionalAuth(sigv4Ext.ID()))

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
	processors.Set(batchprocessor.NewTranslator(common.WithName("opentelemetry_traces"), batchprocessor.WithSendBatchSize(common.MaxSpansPerRequest), batchprocessor.WithSendBatchMaxSize(common.MaxSpansPerRequest), batchprocessor.WithTimeout(common.BatchTimeout)))

	exporters := common.NewTranslatorMap[component.Config, component.ID](otlphttp.NewTranslatorWithName("traces", otlphttp.EndpointConfig{TracesEndpoint: tracesEndpoint}, otlphttp.WithAuthenticator(agentHealthExt.ID())))
	connectors := common.NewTranslatorMap[component.Config, component.ID](fwdConnector)

	if common.GetOrDefaultBool(conf, common.OtelSpanMetricsEnabledKey, false) {
		sm := spanmetrics.NewTranslator(common.OpenTelemetryKey)
		exporters.Set(sm)
		connectors.Set(sm)
	}

	return &common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap[component.Config, component.ID](fwdConnector),
		Processors: processors,
		Exporters:  exporters,
		Extensions: common.NewTranslatorMap[component.Config, component.ID](sigv4Ext, agentHealthExt),
		Connectors: connectors,
	}, nil
}
