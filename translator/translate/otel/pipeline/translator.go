// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pipeline

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/service/pipelines"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

var (
	ErrNoPipelines = errors.New("no valid pipelines")
)

type Translator = common.PipelineTranslator

type TranslatorMap = common.TranslatorMap[*common.ComponentTranslators, pipeline.ID]

type Translation struct {
	// Pipelines is a map of pipeline IDs to service pipelines.
	Pipelines   pipelines.Config
	Translators common.ComponentTranslators
}

type translator struct {
	translators common.TranslatorMap[*common.ComponentTranslators, pipeline.ID]
}

var _ common.Translator[*Translation, component.ID] = (*translator)(nil) // ID doesn't really matter

func NewTranslator(translators common.TranslatorMap[*common.ComponentTranslators, pipeline.ID]) common.Translator[*Translation, component.ID] {
	return &translator{translators: translators}
}

func (t *translator) ID() component.ID {
	newType, _ := component.NewType("")
	return component.NewID(newType)
}

// Translate creates the pipeline configuration.
func (t *translator) Translate(conf *confmap.Conf) (*Translation, error) {
	translation := Translation{
		Pipelines: make(pipelines.Config),
		Translators: common.ComponentTranslators{
			Receivers:  common.NewTranslatorMap[component.Config, component.ID](),
			Processors: common.NewTranslatorMap[component.Config, component.ID](),
			Exporters:  common.NewTranslatorMap[component.Config, component.ID](),
			Extensions: common.NewTranslatorMap[component.Config, component.ID](),
		},
	}
	t.translators.Range(func(pt common.PipelineTranslator) {
		if pipeline, _ := pt.Translate(conf); pipeline != nil {
			translation.Pipelines[pt.ID()] = &pipelines.PipelineConfig{
				Receivers:  pipeline.Receivers.Keys(),
				Processors: pipeline.Processors.Keys(),
				Exporters:  pipeline.Exporters.Keys(),
			}
			translation.Translators.Receivers.Merge(pipeline.Receivers)
			translation.Translators.Processors.Merge(pipeline.Processors)
			translation.Translators.Exporters.Merge(pipeline.Exporters)
			translation.Translators.Extensions.Merge(pipeline.Extensions)
		}
	})
	if len(translation.Pipelines) == 0 {
		return nil, ErrNoPipelines
	}
	return &translation, nil
}
