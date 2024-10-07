// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package host

import (
	"fmt"
	"log"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/cumulativetodeltaprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/ec2taggerprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/metricsdecorator"
)

type translator struct {
	name       string
	receivers  common.TranslatorMap[component.Config]
	processors common.TranslatorMap[component.Config]
	exporters  common.TranslatorMap[component.Config]
	extensions common.TranslatorMap[component.Config]
}

var _ common.Translator[*common.ComponentTranslators] = (*translator)(nil)

// NewTranslator creates a new host pipeline translator. The receiver types
// passed in are converted to config.ComponentIDs, sorted, and used directly
// in the translated pipeline.
func NewTranslator(
	name string,
	receivers common.TranslatorMap[component.Config],
	processors common.TranslatorMap[component.Config],
	exporters common.TranslatorMap[component.Config],
	extensions common.TranslatorMap[component.Config],
) common.Translator[*common.ComponentTranslators] {
	return &translator{name, receivers, processors, exporters, extensions}
}

func (t translator) ID() component.ID {
	return component.NewIDWithName(component.DataTypeMetrics, t.name)
}

// Translate creates a pipeline if metrics section exists.
func (t translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || (!conf.IsSet(common.MetricsKey) && !conf.IsSet(common.ConfigKey(common.LogsKey, common.MetricsCollectedKey))) {
		return nil, &common.MissingKeyError{
			ID:      t.ID(),
			JsonKey: fmt.Sprint(common.MetricsKey, " or ", common.ConfigKey(common.LogsKey, common.MetricsCollectedKey)),
		}
	} else if t.receivers.Len() == 0 || t.exporters.Len() == 0 {
		log.Printf("D! pipeline %s has no receivers/exporters", t.name)
		return nil, nil
	}

	translators := common.ComponentTranslators{
		Receivers:  t.receivers,
		Processors: t.processors,
		Exporters:  t.exporters,
		Extensions: t.extensions,
	}

	if strings.HasPrefix(t.name, common.PipelineNameHostDeltaMetrics) {
		log.Printf("D! delta processor required because metrics with diskio or net are set")
		translators.Processors.Set(cumulativetodeltaprocessor.NewTranslator(common.WithName(t.name), cumulativetodeltaprocessor.WithDefaultKeys()))
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
	return &translators, nil
}
