// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pipeline

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/service"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/util"
)

type testTranslator struct {
	result common.Pipeline
}

var _ common.Translator[common.Pipeline] = (*testTranslator)(nil)

func (t testTranslator) Translate(_ *confmap.Conf) (common.Pipeline, error) {
	return t.result, nil
}

func (t testTranslator) Type() config.Type {
	return ""
}

func TestTranslator(t *testing.T) {
	pt := NewTranslator().(*translator)
	require.EqualValues(t, "", pt.Type())
	pt.translators = nil
	got, err := pt.Translate(confmap.New())
	require.Equal(t, errNoPipelines, err)
	require.Nil(t, got)
	pt.translators = []common.Translator[common.Pipeline]{
		testTranslator{
			result: util.NewPair(config.NewComponentID(""), &service.ConfigServicePipeline{}),
		},
	}
	got, err = pt.Translate(confmap.New())
	require.NoError(t, err)
	require.NotNil(t, got)
}
