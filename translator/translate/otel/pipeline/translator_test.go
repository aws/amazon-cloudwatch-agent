// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pipeline

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type testTranslator struct {
	result *common.ComponentTranslators
}

var _ common.PipelineTranslator = (*testTranslator)(nil)

func (t testTranslator) Translate(_ *confmap.Conf) (*common.ComponentTranslators, error) {
	return t.result, nil
}

func (t testTranslator) ID() pipeline.ID {
	return pipeline.NewIDWithName(pipeline.SignalLogs, "test")
}

func TestTranslator(t *testing.T) {
	pt := NewTranslator(common.NewTranslatorMap[*common.ComponentTranslators, pipeline.ID]())
	got, err := pt.Translate(confmap.New())
	require.Equal(t, ErrNoPipelines, err)
	require.Nil(t, got)
	pt = NewTranslator(common.NewTranslatorMap[*common.ComponentTranslators](&testTranslator{
		result: &common.ComponentTranslators{
			Receivers:  common.NewTranslatorMap[component.Config, component.ID](),
			Processors: common.NewTranslatorMap[component.Config, component.ID](),
			Exporters:  common.NewTranslatorMap[component.Config, component.ID](),
			Extensions: common.NewTranslatorMap[component.Config, component.ID](),
		},
	}))
	got, err = pt.Translate(confmap.New())
	require.NoError(t, err)
	require.NotNil(t, got)
}
