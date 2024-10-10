// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package containerinsightsjmx

import (
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awsemf"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/debug"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/cumulativetodeltaprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/filterprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/jmxtransformprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/metricstransformprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/resourcedetection"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/resourceprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/otlp"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

var (
	baseKey = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey)
	eksKey  = common.ConfigKey(baseKey, common.KubernetesKey)
	jmxKey  = common.ConfigKey(eksKey, "jmx_container_insights")
)

type translator struct {
}

var _ common.Translator[*common.ComponentTranslators] = (*translator)(nil)

func NewTranslator() common.Translator[*common.ComponentTranslators] {
	return &translator{}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(component.DataTypeMetrics, common.PipelineNameContainerInsightsJmx)
}

// Translate creates a pipeline for container insights if the logs.metrics_collected.ecs or logs.metrics_collected.kubernetes
// section is present.
func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || (!conf.IsSet(jmxKey)) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: fmt.Sprint(jmxKey)}
	}
	if val, _ := common.GetBool(conf, jmxKey); !val {
		return nil, nil
	}

	translators := common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap[component.Config](),
		Processors: common.NewTranslatorMap[component.Config](),
		Exporters:  common.NewTranslatorMap[component.Config](),
		Extensions: common.NewTranslatorMap[component.Config](),
	}

	translators.Receivers.Set(otlp.NewTranslator(common.WithName(common.PipelineNameJmx)))
	translators.Processors.Set(filterprocessor.NewTranslator(common.WithName(common.PipelineNameContainerInsightsJmx)))   //Filter metrics
	translators.Processors.Set(resourcedetection.NewTranslatorWithName(common.PipelineNameContainerInsightsJmx))          //Adds k8s cluster/nodename name
	translators.Processors.Set(resourceprocessor.NewTranslator(common.WithName(common.PipelineNameContainerInsightsJmx))) //Change resource attribute names
	translators.Processors.Set(jmxtransformprocessor.NewTranslatorWithName(common.PipelineNameContainerInsightsJmx))      //Removes attributes that are not of [ClusterName, Namespace]
	translators.Processors.Set(metricstransformprocessor.NewTranslatorWithName(common.PipelineNameContainerInsightsJmx))  //Renames metrics and adds pool and area dimensions
	translators.Processors.Set(cumulativetodeltaprocessor.NewTranslator(common.WithName(common.PipelineNameContainerInsightsJmx), cumulativetodeltaprocessor.WithConfigKeys(jmxKey)))
	translators.Exporters.Set(debug.NewTranslator())                                                 //Provides debug info for metrics
	translators.Exporters.Set(awsemf.NewTranslatorWithName(common.PipelineNameContainerInsightsJmx)) //Sends metrics to cloudwatch console (also adds resource attributes to metrics)

	return &translators, nil

}
