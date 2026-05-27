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
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/otlphttp"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/sigv4auth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/resourcedetection"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/systemmetrics"
)

const (
	pipelineNameHostInsights = "host_insights"
)

var hostInsightsKey = common.ConfigKey(common.OpenTelemetryKey, common.CollectKey, common.HostInsightsKey)

type hostInsightsTranslator struct{}

var _ common.PipelineTranslator = (*hostInsightsTranslator)(nil)

func NewHostInsightsTranslator() common.PipelineTranslator {
	return &hostInsightsTranslator{}
}

func (t *hostInsightsTranslator) ID() pipeline.ID {
	return pipeline.NewIDWithName(pipeline.SignalMetrics, pipelineNameHostInsights)
}

func (t *hostInsightsTranslator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || !conf.IsSet(hostInsightsKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: hostInsightsKey}
	}

	region := agent.Global_Config.Region
	if region == "" {
		return nil, fmt.Errorf("region is required for %s pipeline", pipelineNameHostInsights)
	}
	metricsEndpoint := serviceEndpoint("monitoring", region, "/v1/metrics")

	sigv4Ext := sigv4auth.NewTranslatorWithService("monitoring")

	return &common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap[component.Config, component.ID](systemmetrics.NewTranslator()),
		Processors: common.NewTranslatorMap[component.Config, component.ID](resourcedetection.NewTranslator()),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](otlphttp.NewTranslatorWithName(pipelineNameHostInsights, otlphttp.EndpointConfig{MetricsEndpoint: metricsEndpoint}, otlphttp.WithAuthenticator(sigv4Ext.ID()))),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](sigv4Ext),
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
