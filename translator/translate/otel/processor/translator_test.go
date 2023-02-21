// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package processor

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
)

func TestTranslator(t *testing.T) {
	factory := componenttest.NewNopProcessorFactory()
	got := NewDefaultTranslator(factory)
	require.Equal(t, component.Type("nop"), got.Type())
	cfg, err := got.Translate(nil, common.TranslatorOptions{})
	require.NoError(t, err)
	require.Equal(t, factory.CreateDefaultConfig(), cfg)
}
