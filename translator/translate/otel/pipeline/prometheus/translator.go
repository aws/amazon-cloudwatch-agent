// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"go.opentelemetry.io/collector/config"
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

func (t *translator) Type() config.Type {
	return pipelineName
}

// Translate creates a pipeline for prometheus if the logs.metrics_collected.prometheus
// section is present.
func (t *translator) Translate(conf *confmap.Conf) (common.Pipeline, error) {
	key := common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.PrometheusKey)
	if conf == nil || !conf.IsSet(key) {
		return nil, &common.MissingKeyError{Type: t.Type(), JsonKey: key}
	}
	id := config.NewComponentIDWithName(config.MetricsDataType, pipelineName)
	pipeline := &service.ConfigServicePipeline{
		Receivers: []config.ComponentID{config.NewComponentIDWithName("prometheus", pipelineName)},
		Processors: []config.ComponentID{
			config.NewComponentIDWithName("batch", pipelineName),
			config.NewComponentIDWithName("resource", pipelineName),
			config.NewComponentIDWithName("metricstransform", pipelineName),
		},
		Exporters: []config.ComponentID{config.NewComponentIDWithName("awsemf", pipelineName)},
	}
	return collections.NewPair(id, pipeline), nil
}
