// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pipeline

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type testTranslator struct {
	result *common.ComponentTranslators
}

var _ common.Translator[*common.ComponentTranslators] = (*testTranslator)(nil)

func (t testTranslator) Translate(_ *confmap.Conf) (*common.ComponentTranslators, error) {
	return t.result, nil
}

func (t testTranslator) ID() component.ID {
	newType, _ := component.NewType("")
	return component.NewID(newType)
}

func TestTranslator(t *testing.T) {
	pt := NewTranslator(common.NewTranslatorMap[*common.ComponentTranslators]())
	got, err := pt.Translate(confmap.New())
	require.Equal(t, ErrNoPipelines, err)
	require.Nil(t, got)
	pt = NewTranslator(common.NewTranslatorMap[*common.ComponentTranslators](&testTranslator{
		result: &common.ComponentTranslators{
			Receivers:  common.NewTranslatorMap[component.Config](),
			Processors: common.NewTranslatorMap[component.Config](),
			Exporters:  common.NewTranslatorMap[component.Config](),
			Extensions: common.NewTranslatorMap[component.Config](),
		},
	}))
	got, err = pt.Translate(confmap.New())
	require.NoError(t, err)
	require.NotNil(t, got)
}
