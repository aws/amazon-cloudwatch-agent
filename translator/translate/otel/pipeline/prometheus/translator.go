// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/service"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/internal/util/collections"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

const (
	pipelineName = "prometheus"
)

type translator struct {
}

var _ common.Translator[common.Pipeline] = (*translator)(nil)

func NewTranslator() common.Translator[common.Pipeline] {
	return &translator{}
}

func (t *translator) Type() component.Type {
	return pipelineName
}

// Translate creates a pipeline for prometheus if the logs.metrics_collected.prometheus
// section is present.
func (t *translator) Translate(conf *confmap.Conf, _ common.TranslatorOptions) (common.Pipeline, error) {
	key := common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.PrometheusKey)
	if conf == nil || !conf.IsSet(key) {
		return nil, &common.MissingKeyError{Type: t.Type(), JsonKey: key}
	}
	id := component.NewIDWithName(component.DataTypeMetrics, pipelineName)
	pipeline := &service.ConfigServicePipeline{
		Receivers: []component.ID{component.NewIDWithName("prometheus", pipelineName)},
		Processors: []component.ID{
			component.NewIDWithName("batch", pipelineName),
			component.NewIDWithName("resource", pipelineName),
			component.NewIDWithName("metricstransform", pipelineName),
		},
		Exporters: []component.ID{component.NewIDWithName("awsemf", pipelineName)},
	}
	return collections.NewPair(id, pipeline), nil
}
