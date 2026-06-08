// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchlogsprovisionerextension

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestNewFactory(t *testing.T) {
	f := NewFactory()
	assert.NotNil(t, f)
	assert.Equal(t, "awscloudwatchlogsprovisioner", f.Type().String())
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.Equal(t, 10*time.Second, cfg.LogsProvisionTimeout)
	assert.Equal(t, 30*time.Second, cfg.LogsProvisionFailureBackoff)
	assert.Nil(t, cfg.AdditionalAuth)
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateExtension(t *testing.T) {
	f := NewFactory()
	cfg := createDefaultConfig()
	ext, err := f.Create(t.Context(), extensiontest.NewNopSettings(f.Type()), cfg)
	require.NoError(t, err)
	assert.NotNil(t, ext)
}
