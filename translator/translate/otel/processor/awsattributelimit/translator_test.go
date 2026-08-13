// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsattributelimit

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsattributelimitprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	tt := NewTranslator(common.WithName("opentelemetry_metrics"))
	assert.Equal(t, "awsattributelimit/opentelemetry_metrics", tt.ID().String())

	got, err := tt.Translate(nil)
	require.NoError(t, err)
	cfg := got.(*awsattributelimitprocessor.Config)
	assert.Equal(t, 150, cfg.MaxTotalAttributes)
	assert.Empty(t, cfg.UnconditionalRemovalKeys)
	assert.Empty(t, cfg.UnconditionalRemovalPrefixes)
	require.NoError(t, cfg.Validate())
}
