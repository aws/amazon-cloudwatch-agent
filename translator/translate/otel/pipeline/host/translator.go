// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package host

import (
	"log"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/sigv4auth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/batchprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/cumulativetodeltaprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/ec2taggerprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/metricsdecorator"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/rollupprocessor"
)

type translator struct {
	name      string
	receivers common.TranslatorMap[component.Config]
	exporters common.TranslatorMap[component.Config]
}

var _ common.Translator[*common.ComponentTranslators] = (*translator)(nil)

// NewTranslator creates a new host pipeline translator. The receiver types
// passed in are converted to config.ComponentIDs, sorted, and used directly
// in the translated pipeline.
func NewTranslator(
	name string,
	receivers common.TranslatorMap[component.Config],
	exporters common.TranslatorMap[component.Config],
) common.Translator[*common.ComponentTranslators] {
	return &translator{name, receivers, exporters}
}

func (t translator) ID() component.ID {
	return component.NewIDWithName(component.DataTypeMetrics, t.name)
}

// Translate creates a pipeline if metrics section exists.
func (t translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || !conf.IsSet(common.MetricsKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: common.MetricsKey}
	} else if t.receivers.Len() == 0 || t.exporters.Len() == 0 {
		log.Printf("D! pipeline %s has no receivers/exporters", t.name)
		return nil, nil
	}

	translators := common.ComponentTranslators{
		Receivers:  t.receivers,
		Processors: common.NewTranslatorMap[component.Config](),
		Exporters:  t.exporters,
		Extensions: common.NewTranslatorMap[component.Config](),
	}

	// we need to add delta processor because (only) diskio and net input plugins report delta metric
	if strings.HasPrefix(t.name, common.PipelineNameHostDeltaMetrics) {
		log.Printf("D! delta processor required because metrics with diskio or net are set")
		translators.Processors.Set(cumulativetodeltaprocessor.NewTranslatorWithName(t.name))
	}

	if conf.IsSet(common.ConfigKey(common.MetricsKey, common.AppendDimensionsKey)) {
		log.Printf("D! ec2tagger processor required because append_dimensions is set")
		translators.Processors.Set(ec2taggerprocessor.NewTranslator())
	}

	mdt := metricsdecorator.NewTranslator(metricsdecorator.WithIgnorePlugins(common.JmxKey))
	if mdt.IsSet(conf) {
		log.Printf("D! metric decorator required because measurement fields are set")
		translators.Processors.Set(mdt)
	}

	_, ok1 := t.exporters.Get(component.NewID(component.MustNewType("prometheusremotewrite")))
	_, ok2 := t.exporters.Get(component.MustNewIDWithName("prometheusremotewrite", "amp"))

	if ok1 || ok2 {
		translators.Extensions.Set(sigv4auth.NewTranslator())
	}

	if (ok1 || ok2) && conf.IsSet(common.MetricsAggregationDimensionsKey) {
		translators.Processors.Set(rollupprocessor.NewTranslator())
	}

	if _, ok := t.exporters.Get(component.NewID(component.MustNewType("awscloudwatch"))); !ok {
		translators.Processors.Set(batchprocessor.NewTranslatorWithNameAndSection(t.name, common.MetricsKey))
	} else {
		// only add agenthealth for the cloudwatch exporter
		translators.Extensions.Set(agenthealth.NewTranslator(component.DataTypeMetrics, []string{agenthealth.OperationPutMetricData}))
	}
	return &translators, nil
}
