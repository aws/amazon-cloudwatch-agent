// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/prometheus"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awsemf"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/batchprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/adapter"
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
		Receivers: common.NewTranslatorMap(adapter.NewTranslator(prometheus.SectionKey, key, time.Minute)),
		Processors: common.NewTranslatorMap(
			batchprocessor.NewTranslatorWithNameAndSection(pipelineName, common.LogsKey), // prometheus sits under metrics_collected in "logs"
		),
		Exporters:  common.NewTranslatorMap(awsemf.NewTranslatorWithName(pipelineName)),
		Extensions: common.NewTranslatorMap(agenthealth.NewTranslator(component.DataTypeLogs, []string{agenthealth.OperationPutLogEvents})),
	}, nil
}
