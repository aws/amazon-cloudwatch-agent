// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package opentelemetry

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws/endpoints"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/connector/forward"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/otlphttp"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/sigv4auth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/batchprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/resourcedetection"
)

const pipelineNameBaseMetrics = "opentelemetry"

var otelCollectKey = common.ConfigKey(common.OpenTelemetryKey, common.CollectKey)

type baseMetricsTranslator struct{}

var _ common.PipelineTranslator = (*baseMetricsTranslator)(nil)

func NewBaseMetricsTranslator() common.PipelineTranslator {
	return &baseMetricsTranslator{}
}

func (t *baseMetricsTranslator) ID() pipeline.ID {
	return pipeline.NewIDWithName(pipeline.SignalMetrics, pipelineNameBaseMetrics)
}

// Translate creates the shared metrics export pipeline. It activates when any
// collect sub-section is present; it receives data via the forward connector
// from feature pipelines (host_insights, otlp, span_metrics).
func (t *baseMetricsTranslator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || !conf.IsSet(otelCollectKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: otelCollectKey}
	}

	region := agent.Global_Config.Region
	if region == "" {
		return nil, fmt.Errorf("region is required for %s pipeline", pipelineNameBaseMetrics)
	}
	metricsEndpoint := serviceEndpoint("monitoring", region, "/v1/metrics")
	sigv4Ext := sigv4auth.NewTranslatorWithService("monitoring")

	fwdConnector := forward.NewTranslator("otel")

	return &common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap[component.Config, component.ID](fwdConnector),
		Processors: common.NewTranslatorMap[component.Config, component.ID](resourcedetection.NewTranslator(), batchprocessor.NewTranslator(common.WithName(pipelineNameBaseMetrics))),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](otlphttp.NewTranslatorWithName(pipelineNameBaseMetrics, otlphttp.EndpointConfig{MetricsEndpoint: metricsEndpoint}, otlphttp.WithAuthenticator(sigv4Ext.ID()))),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](sigv4Ext),
		Connectors: common.NewTranslatorMap[component.Config, component.ID](fwdConnector),
	}, nil
}

func serviceEndpoint(service, region, path string) string {
	partition, _ := endpoints.PartitionForRegion(endpoints.DefaultPartitions(), region)
	dnsSuffix := partition.DNSSuffix()
	if dnsSuffix == "" {
		dnsSuffix = "amazonaws.com"
	}
	return fmt.Sprintf("https://%s.%s.%s%s", service, region, dnsSuffix, path)
}
