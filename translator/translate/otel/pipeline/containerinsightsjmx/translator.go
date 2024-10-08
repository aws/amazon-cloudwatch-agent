// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package containerinsightsjmx

import (
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awsemf"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/debug"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/cumulativetodeltaprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/jmxfilterprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/jmxtransformprocessor"
	metricstransformprocessorjmx "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/metrictransformprocessorjmx"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/resourcedetectionjmx"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/resourceprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/otlp"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	pipelineName = "containerinsightsjmx"
	clusterName  = "cluster_name"
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
	return component.NewIDWithName(component.DataTypeMetrics, pipelineName)
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

	translators.Receivers.Set(otlp.NewTranslatorWithName(common.JmxKey))
	translators.Processors.Set(jmxfilterprocessor.NewTranslatorWithName(pipelineName))                     //Filter metrics
	translators.Processors.Set(resourcedetectionjmx.NewTranslator())                                       //Adds k8s cluster/nodename name
	translators.Processors.Set(resourceprocessor.NewTranslator(resourceprocessor.WithName("jmxResource"))) //Change resource attribute names
	translators.Processors.Set(jmxtransformprocessor.NewTranslatorWithName(pipelineName))                  //Removes attributes that are not of [ClusterName, Namespace]
	translators.Processors.Set(metricstransformprocessorjmx.NewTranslatorWithName(pipelineName))           //Renames metrics and adds pool and area dimensions
	translators.Processors.Set(cumulativetodeltaprocessor.NewTranslator(common.WithName(pipelineName), cumulativetodeltaprocessor.WithConfigKeys(jmxKey)))
	translators.Exporters.Set(debug.NewTranslator())                      //Provides debug info for metrics
	translators.Exporters.Set(awsemf.NewTranslatorWithName(pipelineName)) //Sends metrics to cloudwatch console (also adds resource attributes to metrics)

	return &translators, nil

}
