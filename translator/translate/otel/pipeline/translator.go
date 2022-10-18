// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pipeline

import (
	"errors"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/pipeline/containerinsights"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/pipeline/host"
)

var (
	errNoPipelines = errors.New("no valid pipelines")
)

type translator struct {
	translators []common.Translator[common.Pipeline]
}

var _ common.Translator[common.Pipelines] = (*translator)(nil)

func NewTranslator() common.Translator[common.Pipelines] {
	return &translator{
		translators: []common.Translator[common.Pipeline]{
			host.NewTranslator(),
			containerinsights.NewTranslator(),
		},
	}
}

// Type is unused.
func (t *translator) Type() config.Type {
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
		return nil, errNoPipelines
	}
	return pipelines, nil
}
