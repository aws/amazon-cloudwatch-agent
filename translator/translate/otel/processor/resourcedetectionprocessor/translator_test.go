// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resourcedetectionprocessor

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

func TestTranslate_AzureWithOTelPlaceholders(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"metrics": map[string]interface{}{
			"append_dimensions": map[string]interface{}{
				"InstanceId": "${host.id}",
				"Region":     "${cloud.region}",
				"VMSize":     "${azure.vm.size}",
			},
		},
	})

	tr := NewTranslator([]string{"azure", "system"})
	cfg, err := tr.Translate(conf)
	require.NoError(t, err)

	rdCfg := cfg.(*resourcedetectionprocessor.Config)
	az := rdCfg.DetectorConfig.AzureConfig.ResourceAttributes

	assert.True(t, az.HostID.Enabled)
	assert.True(t, az.CloudRegion.Enabled)
	assert.True(t, az.AzureVMSize.Enabled)
	assert.False(t, az.AzureResourcegroupName.Enabled)
	assert.False(t, az.AzureVMScalesetName.Enabled)
	assert.False(t, az.CloudPlatform.Enabled)
}

func TestTranslate_NoAppendDimensions(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"metrics": map[string]interface{}{},
	})

	tr := NewTranslator([]string{"azure"})
	cfg, err := tr.Translate(conf)
	require.NoError(t, err)

	rdCfg := cfg.(*resourcedetectionprocessor.Config)
	az := rdCfg.DetectorConfig.AzureConfig.ResourceAttributes

	assert.False(t, az.HostID.Enabled)
	assert.False(t, az.CloudRegion.Enabled)
}

func TestTranslate_Detectors(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]interface{}{})

	tr := NewTranslator([]string{"azure", "gcp", "system"})
	cfg, err := tr.Translate(conf)
	require.NoError(t, err)

	rdCfg := cfg.(*resourcedetectionprocessor.Config)
	assert.Equal(t, []string{"azure", "gcp", "system"}, rdCfg.Detectors)
	assert.False(t, rdCfg.Override)
}
