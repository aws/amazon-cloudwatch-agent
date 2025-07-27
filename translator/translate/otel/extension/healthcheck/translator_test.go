// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package healthcheck

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"
)

func TestHealthCheckTranslator(t *testing.T) {
	translator := NewHealthCheckTranslator()
	assert.Equal(t, "health_check", translator.ID().Type().String())

	conf := confmap.New()
	cfg, err := translator.Translate(conf)
	assert.NoError(t, err)

	// Assert the config has the expected fields
	healthCheckCfg, ok := cfg.(*struct {
		Endpoint string `mapstructure:"endpoint"`
		Path     string `mapstructure:"path"`
	})
	assert.True(t, ok)
	assert.Equal(t, "0.0.0.0:13133", healthCheckCfg.Endpoint)
	assert.Equal(t, "/", healthCheckCfg.Path)
}
