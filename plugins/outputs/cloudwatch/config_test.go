// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT
package cloudwatch

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/service/servicetest"
)

// TestConfig will verify various config files can be loaded.
// Verifies Config.Validate() implicitly.
func TestConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)
	factory := NewFactory()
	factories.Exporters[TypeStr] = factory

	// Test missing region.
	// Expect invalid because factory does not have a default value.
	fp := filepath.Join("testdata", "missing_region.yaml")
	_, err = servicetest.LoadConfigAndValidate(fp, factories)
	assert.Error(t, err)

	// Test small force flush interval.
	// Expect invalid because of minimum duration check.
	// A value of 60 in YAML will be parsed as 60ns.
	fp = filepath.Join("testdata", "small_force_flush_interval.yaml")
	_, err = servicetest.LoadConfigAndValidate(fp, factories)
	assert.Error(t, err)

	// Test missing namespace.
	// Expect valid because factory has a default value.
	fp = filepath.Join("testdata", "missing_namespace.yaml")
	_, err = servicetest.LoadConfigAndValidate(fp, factories)
	assert.NoError(t, err)

	// Test minimal valid.
	fp = filepath.Join("testdata", "minimal.yaml")
	c, err := servicetest.LoadConfigAndValidate(fp, factories)
	assert.NoError(t, err)
	assert.NotNil(t, c)
	assert.Equal(t, 1, len(c.Exporters))

	// Test full valid.
	fp = filepath.Join("testdata", "full.yaml")
	c, err = servicetest.LoadConfigAndValidate(fp, factories)
	assert.NoError(t, err)
	assert.NotNil(t, c)
	assert.Equal(t, 1, len(c.Exporters))
	c2 := c.Exporters[component.NewID(TypeStr)].(*Config)
	assert.Equal(t, "val1", c2.Namespace)
	assert.Equal(t, "val2", c2.Region)
	assert.Equal(t, "val3", c2.EndpointOverride)
	assert.Equal(t, "val4", c2.AccessKey)
	assert.Equal(t, "val5", c2.SecretKey)
	assert.Equal(t, "val6", c2.RoleARN)
	assert.Equal(t, "val7", c2.Profile)
	assert.Equal(t, "val8", c2.SharedCredentialFilename)
	assert.Equal(t, "val9", c2.Token)
	assert.Equal(t, 7, c2.MaxDatumsPerCall)
	assert.Equal(t, 9, c2.MaxValuesPerDatum)
	assert.Equal(t, 60*time.Second, c2.ForceFlushInterval)
	// todo: verify MetricDecorations, RollupDimensions, DropOriginConfigs
}
