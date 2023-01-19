// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pipeline

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

var (
	ErrNoPipelines = errors.New("no valid pipelines")
)

type translator struct {
	translators []common.Translator[common.Pipeline]
}

var _ common.Translator[common.Pipelines] = (*translator)(nil)

func NewTranslator(translators ...common.Translator[common.Pipeline]) common.Translator[common.Pipelines] {
	return &translator{translators}
}

// Type is unused.
func (t *translator) Type() component.Type {
	return ""
}

// Translate creates the pipeline configuration.
func (t *translator) Translate(conf *confmap.Conf) (common.Pipelines, error) {
	pipelines := make(common.Pipelines)
	for _, pt := range t.translators {
		if pipeline, _ := pt.Translate(conf); pipeline != nil {
			pipelines[pipeline.Key] = pipeline.Value
		}
	}
	if len(pipelines) == 0 {
		return nil, ErrNoPipelines
	}
	return pipelines, nil
}
