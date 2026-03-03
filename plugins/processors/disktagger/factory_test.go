// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package disktagger

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/internal/cloudprovider"
)

func TestNewFactory(t *testing.T) {
	f := NewFactory()
	require.NotNil(t, f)
	assert.Equal(t, "disktagger", f.Type().String())
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.Equal(t, 5*time.Minute, cfg.RefreshInterval)
	assert.Equal(t, "device", cfg.DiskDeviceTagKey)
	assert.Equal(t, cloudprovider.Unknown, cfg.CloudProvider)
}

func TestCacheFactory_NoCloud(t *testing.T) {
	factory := newCacheFactory(t.Context())
	cfg := &Config{CloudProvider: cloudprovider.Unknown}
	cache := factory(cfg)
	assert.Nil(t, cache)
}

func TestCacheFactory_Azure(t *testing.T) {
	factory := newCacheFactory(t.Context())
	cfg := &Config{CloudProvider: cloudprovider.Azure}
	cache := factory(cfg)
	require.NotNil(t, cache)
}
