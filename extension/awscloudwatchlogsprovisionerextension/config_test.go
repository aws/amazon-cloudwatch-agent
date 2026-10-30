// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchlogsprovisionerextension

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
)

func TestConfig_Defaults(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	assert.Equal(t, 10*time.Second, cfg.LogsProvisionTimeout)
	assert.Equal(t, 30*time.Second, cfg.LogsProvisionFailureBackoff)
	assert.Nil(t, cfg.AdditionalAuth)
	assert.Empty(t, cfg.Region)
}

func TestConfig_Validate_RequiresRegion(t *testing.T) {
	cfg := &Config{LogsProvisionTimeout: 10 * time.Second}
	assert.Error(t, cfg.Validate(), "should require region")

	cfg.Region = "us-east-1"
	assert.NoError(t, cfg.Validate())
}

func TestConfig_Validate_RequiresPositiveTimeout(t *testing.T) {
	cfg := &Config{Region: "us-east-1", LogsProvisionTimeout: 0}
	assert.Error(t, cfg.Validate(), "should reject zero timeout")

	cfg.LogsProvisionTimeout = -1 * time.Second
	assert.Error(t, cfg.Validate(), "should reject negative timeout")

	cfg.LogsProvisionTimeout = 10 * time.Second
	assert.NoError(t, cfg.Validate())
}

func TestConfig_WithAdditionalAuth(t *testing.T) {
	authID := component.MustNewID("sigv4auth")
	cfg := &Config{
		Region:               "us-east-1",
		LogsProvisionTimeout: 10 * time.Second,
		AdditionalAuth:       &authID,
	}

	assert.Equal(t, "sigv4auth", cfg.AdditionalAuth.String())
	assert.NoError(t, cfg.Validate())
}

func TestFactory_Type(t *testing.T) {
	f := NewFactory()
	assert.Equal(t, component.MustNewType("awscloudwatchlogsprovisioner"), f.Type())
}
