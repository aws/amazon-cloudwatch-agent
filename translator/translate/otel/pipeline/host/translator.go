// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package host

import (
	"fmt"
	"log"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awscloudwatch"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awsemf"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/prometheusremotewrite"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/sigv4auth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/awsentity"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/batchprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/cumulativetodeltaprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/ec2taggerprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/metricsdecorator"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/rollupprocessor"
)

type translator struct {
	name string
	common.DestinationProvider
	receivers common.ComponentTranslatorMap
}

var _ common.PipelineTranslator = (*translator)(nil)

// NewTranslator creates a new host pipeline translator. The receiver types
// passed in are converted to config.ComponentIDs, sorted, and used directly
// in the translated pipeline.
func NewTranslator(
	name string,
	receivers common.ComponentTranslatorMap,
	opts ...common.TranslatorOption,
) common.PipelineTranslator {
	t := &translator{name: name, receivers: receivers}
	for _, opt := range opts {
		opt(t)
	}
	if t.Destination() != "" {
		t.name += "/" + t.Destination()
	}
	return t
}

func (t translator) ID() pipeline.ID {
	return pipeline.NewIDWithName(pipeline.SignalMetrics, t.name)
}

// Translate creates a pipeline if metrics section exists.
func (t translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || t.receivers.Len() == 0 {
		return nil, fmt.Errorf("no receivers configured in pipeline %s", t.name)
	}
	var entityProcessor common.ComponentTranslator
	if strings.HasPrefix(t.name, common.PipelineNameHostOtlpMetrics) {
		entityProcessor = nil
	} else if strings.HasPrefix(t.name, common.PipelineNameHostCustomMetrics) {
		entityProcessor = awsentity.NewTranslatorWithEntityType(awsentity.Service, "telegraf", true)
	} else if strings.HasPrefix(t.name, common.PipelineNameHost) || strings.HasPrefix(t.name, common.PipelineNameHostDeltaMetrics) {
		entityProcessor = awsentity.NewTranslatorWithEntityType(awsentity.Resource, "", false)
	}

	translators := common.ComponentTranslators{
		Receivers:  t.receivers,
		Processors: common.NewTranslatorMap[component.Config, component.ID](),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](),
	}
	currentContext := context.CurrentContext()
	if entityProcessor != nil && currentContext.Mode() == config.ModeEC2 && !currentContext.RunInContainer() && (t.Destination() == common.CloudWatchKey || t.Destination() == common.DefaultDestination) {
		translators.Processors.Set(entityProcessor)
	}

	if strings.HasPrefix(t.name, common.PipelineNameHostDeltaMetrics) || strings.HasPrefix(t.name, common.PipelineNameHostOtlpMetrics) {
		log.Printf("D! delta processor required because metrics with diskio or net are set")
		translators.Processors.Set(cumulativetodeltaprocessor.NewTranslator(common.WithName(t.name), cumulativetodeltaprocessor.WithDefaultKeys()))
	}

	if t.Destination() != common.CloudWatchLogsKey {
		if conf.IsSet(common.ConfigKey(common.MetricsKey, common.AppendDimensionsKey)) {
			log.Printf("D! ec2tagger processor required because append_dimensions is set")
			translators.Processors.Set(ec2taggerprocessor.NewTranslator())
		}

		mdt := metricsdecorator.NewTranslator(metricsdecorator.WithIgnorePlugins(common.JmxKey))
		if mdt.IsSet(conf) {
			log.Printf("D! metric decorator required because measurement fields are set")
			translators.Processors.Set(mdt)
		}
	}

	switch t.Destination() {
	case common.DefaultDestination, common.CloudWatchKey:
		translators.Exporters.Set(awscloudwatch.NewTranslator())
		translators.Extensions.Set(agenthealth.NewTranslator(pipeline.SignalMetrics, []string{agenthealth.OperationPutMetricData}))
	case common.AMPKey:
		if conf.IsSet(common.MetricsAggregationDimensionsKey) {
			translators.Processors.Set(rollupprocessor.NewTranslator())
		}
		translators.Processors.Set(batchprocessor.NewTranslatorWithNameAndSection(t.name, common.MetricsKey))
		translators.Exporters.Set(prometheusremotewrite.NewTranslatorWithName(common.AMPKey))
		translators.Extensions.Set(sigv4auth.NewTranslator())
	case common.CloudWatchLogsKey:
		translators.Processors.Set(batchprocessor.NewTranslatorWithNameAndSection(t.name, common.LogsKey))
		translators.Exporters.Set(awsemf.NewTranslator())
		translators.Extensions.Set(agenthealth.NewTranslator(pipeline.SignalLogs, []string{agenthealth.OperationPutLogEvents}))
	default:
		return nil, fmt.Errorf("pipeline (%s) does not support destination (%s) in configuration", t.name, t.Destination())
	}

	return &translators, nil
}
