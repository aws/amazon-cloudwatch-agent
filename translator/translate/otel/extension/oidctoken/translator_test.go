// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package oidctoken

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/oidctokenextension"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
)

func TestTranslator(t *testing.T) {
	t.Cleanup(context.ResetContext)
	context.ResetContext()
	context.CurrentContext().SetMode(config.ModeAzureVM)

	tt := NewTranslator()
	assert.Equal(t, "oidctoken", tt.ID().String())

	got, err := tt.Translate(confmap.New())
	require.NoError(t, err)
	require.NotNil(t, got)

	cfg, ok := got.(*oidctokenextension.Config)
	require.True(t, ok)
	assert.Equal(t, oidctokenextension.ProviderAzure, cfg.Provider)
	assert.Equal(t, paths.OIDCTokenPath, cfg.OutputTokenFile)
	assert.NoError(t, cfg.Validate())
}
