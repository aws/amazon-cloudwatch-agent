// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package containerinsights

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor/batchprocessor"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/exporter/awsemf"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/processor"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/receiver/awscontainerinsight"
)

const (
	pipelineName = "containerinsights"
)

var (
	baseKey = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey)
	eksKey  = common.ConfigKey(baseKey, common.ECSKey)
	ecsKey  = common.ConfigKey(baseKey, common.ECSKey)
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

// Translate creates a pipeline for container insights if the logs.metrics_collected.ecs
// section is present.
func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || (!conf.IsSet(ecsKey) && !conf.IsSet(eksKey)) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: fmt.Sprint(ecsKey, " or ", eksKey)}
	}
	return &common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap(awscontainerinsight.NewTranslator()),
		Processors: common.NewTranslatorMap(processor.NewDefaultTranslatorWithName(pipelineName, batchprocessor.NewFactory())),
		Exporters:  common.NewTranslatorMap(awsemf.NewTranslatorWithName(pipelineName)),
	}, nil
}
