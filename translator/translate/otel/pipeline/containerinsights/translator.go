// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package containerinsights

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/service"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/internal/util/collections"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

const (
	pipelineName = "containerinsights"
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

// Translate creates a pipeline for container insights if the logs.metrics_collected.ecs
// section is present.
func (t *translator) Translate(conf *confmap.Conf) (common.Pipeline, error) {
	key := common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.ECSKey)
	if conf == nil || !conf.IsSet(key) {
		return nil, &common.MissingKeyError{Type: t.Type(), JsonKey: key}
	}
	id := component.NewIDWithName(component.DataTypeMetrics, pipelineName)
	pipeline := &service.ConfigServicePipeline{
		Receivers:  []component.ID{component.NewID("awscontainerinsightreceiver")},
		Processors: []component.ID{component.NewIDWithName("batch", pipelineName)},
		Exporters:  []component.ID{component.NewIDWithName("awsemf", pipelineName)},
	}
	return collections.NewPair(id, pipeline), nil
}
