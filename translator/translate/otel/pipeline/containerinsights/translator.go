// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package containerinsights

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awsemf"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/batchprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/gpu"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/metricstransformprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/awscontainerinsight"
)

const (
	pipelineName = "containerinsights"
)

var (
	baseKey = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey)
	eksKey  = common.ConfigKey(baseKey, common.KubernetesKey)
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

// Translate creates a pipeline for container insights if the logs.metrics_collected.ecs or logs.metrics_collected.kubernetes
// section is present.
func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || (!conf.IsSet(ecsKey) && !conf.IsSet(eksKey)) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: fmt.Sprint(ecsKey, " or ", eksKey)}
	}

	// Append the metricstransformprocessor only if enhanced container insights is enabled
	enhancedContainerInsightsEnabled := awscontainerinsight.EnhancedContainerInsightsEnabled(conf)
	if enhancedContainerInsightsEnabled {
		processors := common.NewTranslatorMap(metricstransformprocessor.NewTranslatorWithName(pipelineName))
		acceleratedComputeMetricsEnabled := awscontainerinsight.AcceleratedComputeMetricsEnabled(conf)
		if acceleratedComputeMetricsEnabled {
			processors.Set(gpu.NewTranslatorWithName(pipelineName))
		}
		processors.Set(batchprocessor.NewTranslatorWithNameAndSection(pipelineName, common.LogsKey))
		return &common.ComponentTranslators{
			Receivers:  common.NewTranslatorMap(awscontainerinsight.NewTranslator()),
			Processors: processors, // EKS & ECS CI sit under metrics_collected in "logs"
			Exporters:  common.NewTranslatorMap(awsemf.NewTranslatorWithName(pipelineName)),
			Extensions: common.NewTranslatorMap(agenthealth.NewTranslator(component.DataTypeLogs, []string{agenthealth.OperationPutLogEvents})),
		}, nil
	}

	return &common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap(awscontainerinsight.NewTranslator()),
		Processors: common.NewTranslatorMap(batchprocessor.NewTranslatorWithNameAndSection(pipelineName, common.LogsKey)), // EKS & ECS CI sit under metrics_collected in "logs"
		Exporters:  common.NewTranslatorMap(awsemf.NewTranslatorWithName(pipelineName)),
		Extensions: common.NewTranslatorMap(agenthealth.NewTranslator(component.DataTypeLogs, []string{agenthealth.OperationPutLogEvents})),
	}, nil
}
