// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package extension

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension/extensiontest"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

type testTranslator struct {
	result component.Config
}

var _ common.Translator[component.Config] = (*testTranslator)(nil)

func (t testTranslator) Translate(_ *confmap.Conf) (component.Config, error) {
	return t.result, nil
}

func (t testTranslator) ID() component.ID {
	return component.NewID("")
}

func TestTranslator(t *testing.T) {
	et := NewTranslator()
	require.EqualValues(t, "", et.ID().String())
	got, err := et.Translate(confmap.New())
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got, 0)
	et = NewTranslator(
		&testTranslator{
			result: extensiontest.NewNopFactory().CreateDefaultConfig(),
		},
	)
	got, err = et.Translate(confmap.New())
	require.NoError(t, err)
	require.Len(t, got, 1)
}
