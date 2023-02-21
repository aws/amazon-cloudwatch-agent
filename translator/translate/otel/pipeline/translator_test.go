// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pipeline

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/service"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/internal/util/collections"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

type testTranslator struct {
	result common.Pipeline
}

var _ common.Translator[common.Pipeline] = (*testTranslator)(nil)

func (t testTranslator) Translate(_ *confmap.Conf, _ common.TranslatorOptions) (common.Pipeline, error) {
	return t.result, nil
}

func (t testTranslator) Type() component.Type {
	return ""
}

func TestTranslator(t *testing.T) {
	pt := NewTranslator()
	require.EqualValues(t, "", pt.Type())
	got, err := pt.Translate(confmap.New(), common.TranslatorOptions{})
	require.Equal(t, ErrNoPipelines, err)
	require.Nil(t, got)
	pt = NewTranslator(
		&testTranslator{
			result: collections.NewPair(component.NewID(""), &service.ConfigServicePipeline{}),
		},
	)
	got, err = pt.Translate(confmap.New(), common.TranslatorOptions{})
	require.NoError(t, err)
	require.NotNil(t, got)
}
