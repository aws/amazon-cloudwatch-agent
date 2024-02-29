// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package host

import (
	"log"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awscloudwatch"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/cumulativetodeltaprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/ec2taggerprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/metricsdecorator"
	otlpReceiver "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/otlp"
)

type translator struct {
	name      string
	receivers common.TranslatorMap[component.Config]
}

var _ common.Translator[*common.ComponentTranslators] = (*translator)(nil)

// NewTranslator creates a new host pipeline translator. The receiver types
// passed in are converted to config.ComponentIDs, sorted, and used directly
// in the translated pipeline.
func NewTranslator(
	name string,
	receivers common.TranslatorMap[component.Config],
) common.Translator[*common.ComponentTranslators] {
	return &translator{name, receivers}
}

func (t translator) ID() component.ID {
	return component.NewIDWithName(component.DataTypeMetrics, t.name)
}

// Translate creates a pipeline if metrics section exists.
func (t translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || !conf.IsSet(common.MetricsKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: common.MetricsKey}
	}

	hostReceivers := t.receivers
	if common.PipelineNameHost == t.name {
		switch v := conf.Get(common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.OtlpKey)).(type) {
		case []interface{}:
			for index, _ := range v {
				hostReceivers.Set(otlpReceiver.NewTranslator(
					otlpReceiver.WithDataType(component.DataTypeMetrics),
					otlpReceiver.WithInstanceNum(index)))
			}
		case map[string]interface{}:
			hostReceivers.Set(otlpReceiver.NewTranslator(otlpReceiver.WithDataType(component.DataTypeMetrics)))
		}
	}

	if hostReceivers.Len() == 0 {
		log.Printf("D! pipeline %s has no receivers", t.name)
		return nil, nil
	}

	translators := common.ComponentTranslators{
		Receivers:  t.receivers,
		Processors: common.NewTranslatorMap[component.Config](),
		Exporters:  common.NewTranslatorMap(awscloudwatch.NewTranslator()),
		Extensions: common.NewTranslatorMap(agenthealth.NewTranslator(component.DataTypeMetrics, []string{agenthealth.OperationPutMetricData})),
	}

	// we need to add delta processor because (only) diskio and net input plugins report delta metric
	if common.PipelineNameHostDeltaMetrics == t.name {
		log.Printf("D! delta processor required because metrics with diskio or net are set")
		translators.Processors.Set(cumulativetodeltaprocessor.NewTranslatorWithName(t.name))
	}

	if conf.IsSet(common.ConfigKey(common.MetricsKey, "append_dimensions")) {
		log.Printf("D! ec2tagger processor required because append_dimensions is set")
		translators.Processors.Set(ec2taggerprocessor.NewTranslator())
	}

	if metricsdecorator.IsSet(conf) {
		log.Printf("D! metric decorator required because measurement fields are set")
		translators.Processors.Set(metricsdecorator.NewTranslator())
	}
	return &translators, nil
}
