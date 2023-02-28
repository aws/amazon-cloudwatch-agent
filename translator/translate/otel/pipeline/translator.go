// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pipeline

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/service"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

var (
	ErrNoPipelines = errors.New("no valid pipelines")
)

type Translation struct {
	// Pipelines is a map of component IDs to service pipelines.
	Pipelines   map[component.ID]*service.ConfigServicePipeline
	Translators common.ComponentTranslators
}

type translator struct {
	translators []common.Translator[*common.ComponentTranslators]
}

var _ common.Translator[*Translation] = (*translator)(nil)

func NewTranslator(translators ...common.Translator[*common.ComponentTranslators]) common.Translator[*Translation] {
	return &translator{translators}
}

func (t *translator) ID() component.ID {
	return component.NewID("")
}

// Translate creates the pipeline configuration.
func (t *translator) Translate(conf *confmap.Conf) (*Translation, error) {
	translation := Translation{
		Pipelines: make(map[component.ID]*service.ConfigServicePipeline),
		Translators: common.ComponentTranslators{
			Receivers:  common.NewTranslatorMap[component.Config](),
			Processors: common.NewTranslatorMap[component.Config](),
			Exporters:  common.NewTranslatorMap[component.Config](),
		},
	}
	for _, pt := range t.translators {
		if pipeline, _ := pt.Translate(conf); pipeline != nil {
			translation.Pipelines[pt.ID()] = &service.ConfigServicePipeline{
				Receivers:  pipeline.Receivers.SortedKeys(),
				Processors: pipeline.Processors.SortedKeys(),
				Exporters:  pipeline.Exporters.SortedKeys(),
			}
			translation.Translators.Receivers.Merge(pipeline.Receivers)
			translation.Translators.Processors.Merge(pipeline.Processors)
			translation.Translators.Exporters.Merge(pipeline.Exporters)
		}
	}
	if len(translation.Pipelines) == 0 {
		return nil, ErrNoPipelines
	}
	return &translation, nil
}
