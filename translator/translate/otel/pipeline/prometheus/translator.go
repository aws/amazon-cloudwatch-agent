// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor/batchprocessor"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/exporter/awsemf"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/extension/ecsobserver"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/processor"
	metricstransformprocessor "github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/processor/metricstransform"
	resourceprocessor "github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/processor/resource"
	prometheusreceiver "github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/receiver/prometheus"
)

const (
	pipelineName = "prometheus"
)

type translator struct {
}

var _ common.Translator[*common.ComponentTranslators] = (*translator)(nil)

func NewTranslator() common.Translator[*common.ComponentTranslators] {
	return &translator{}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(component.DataTypeMetrics, pipelineName)
}

// Translate creates a pipeline for prometheus if the logs.metrics_collected.prometheus
// section is present.
func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	key := common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.PrometheusKey)
	if conf == nil || !conf.IsSet(key) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: key}
	}
	return &common.ComponentTranslators{
		Receivers: common.NewTranslatorMap(prometheusreceiver.NewTranslatorWithName(pipelineName)),
		Processors: common.NewTranslatorMap(
			processor.NewDefaultTranslatorWithName(pipelineName, batchprocessor.NewFactory()),
			metricstransformprocessor.NewTranslatorWithName(pipelineName),
			resourceprocessor.NewTranslatorWithName(pipelineName),
		),
		Exporters:  common.NewTranslatorMap(awsemf.NewTranslatorWithName(pipelineName)),
		Extensions: common.NewTranslatorMap(ecsobserver.NewTranslatorWithName(common.PrometheusKey)),
	}, nil
}
