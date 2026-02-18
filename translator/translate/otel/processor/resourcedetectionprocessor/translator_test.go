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

func TestTranslate_EC2WithLegacyPlaceholders(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"metrics": map[string]interface{}{
			"append_dimensions": map[string]interface{}{
				"InstanceId":   "${aws:InstanceId}",
				"InstanceType": "${aws:InstanceType}",
				"ImageId":      "${aws:ImageId}",
			},
		},
	})

	tr := NewTranslator([]string{"ec2", "system"})
	cfg, err := tr.Translate(conf)
	require.NoError(t, err)

	rdCfg := cfg.(*resourcedetectionprocessor.Config)
	ec2 := rdCfg.DetectorConfig.EC2Config.ResourceAttributes

	assert.True(t, ec2.HostID.Enabled)
	assert.True(t, ec2.HostType.Enabled)
	assert.True(t, ec2.HostImageID.Enabled)
	assert.False(t, ec2.CloudRegion.Enabled)
	assert.False(t, ec2.CloudPlatform.Enabled)
}

func TestTranslate_MixedLegacyAndOTel(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"metrics": map[string]interface{}{
			"append_dimensions": map[string]interface{}{
				"InstanceId": "${aws:InstanceId}",
				"Region":     "${cloud.region}",
			},
		},
	})

	tr := NewTranslator([]string{"ec2"})
	cfg, err := tr.Translate(conf)
	require.NoError(t, err)

	rdCfg := cfg.(*resourcedetectionprocessor.Config)
	ec2 := rdCfg.DetectorConfig.EC2Config.ResourceAttributes

	assert.True(t, ec2.HostID.Enabled)
	assert.True(t, ec2.CloudRegion.Enabled)
	assert.False(t, ec2.HostType.Enabled)
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
