// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsnvme

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/testutil"
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsnvmereceiver"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	tt := NewTranslator()
	assert.EqualValues(t, "awsnvmereceiver", tt.ID().String())
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *confmap.Conf
		wantErr error
	}{
		"WithMissingKey": {
			input: map[string]interface{}{"logs": map[string]interface{}{}},
			wantErr: &common.MissingKeyError{
				ID:      tt.ID(),
				JsonKey: baseKey,
			},
		},
		"WithEmptyConfig": {
			input: testutil.GetJson(t, filepath.Join("testdata", "empty_config.json")),
			want:  testutil.GetConf(t, filepath.Join("testdata", "empty_config.yaml")),
		},
		"WithCompleteConfig": {
			input: testutil.GetJson(t, filepath.Join("testdata", "config.json")),
			want:  testutil.GetConf(t, filepath.Join("testdata", "config.yaml")),
		},
		"WithCustomInterval": {
			input: testutil.GetJson(t, filepath.Join("testdata", "custom_interval.json")),
			want:  testutil.GetConf(t, filepath.Join("testdata", "custom_interval.yaml")),
		},
		"WithAgentInterval": {
			input: testutil.GetJson(t, filepath.Join("testdata", "agent_interval.json")),
			want:  testutil.GetConf(t, filepath.Join("testdata", "agent_interval.yaml")),
		},
		"WithOverrideAgentInterval": {
			input: testutil.GetJson(t, filepath.Join("testdata", "override_agent_interval.json")),
			want:  testutil.GetConf(t, filepath.Join("testdata", "override_agent_interval.yaml")),
		},
		"WithSpecificResources": {
			input: testutil.GetJson(t, filepath.Join("testdata", "specific_resources.json")),
			want:  testutil.GetConf(t, filepath.Join("testdata", "specific_resources.yaml")),
		},
		"WithAllResources": {
			input: testutil.GetJson(t, filepath.Join("testdata", "all_resources.json")),
			want:  testutil.GetConf(t, filepath.Join("testdata", "all_resources.yaml")),
		},
		"WithPrefixedMetrics": {
			input: testutil.GetJson(t, filepath.Join("testdata", "prefixed_metrics.json")),
			want:  testutil.GetConf(t, filepath.Join("testdata", "prefixed_metrics.yaml")),
		},
		"WithNonPrefixedMetrics": {
			input: testutil.GetJson(t, filepath.Join("testdata", "non_prefixed_metrics.json")),
			want:  testutil.GetConf(t, filepath.Join("testdata", "non_prefixed_metrics.yaml")),
		},
		"WithEmptyMeasurements": {
			input: testutil.GetJson(t, filepath.Join("testdata", "empty_measurements.json")),
			want:  testutil.GetConf(t, filepath.Join("testdata", "empty_measurements.yaml")),
		},
		"WithNoMeasurements": {
			input: testutil.GetJson(t, filepath.Join("testdata", "no_measurements.json")),
			want:  testutil.GetConf(t, filepath.Join("testdata", "no_measurements.yaml")),
		},
		"WithMixedMetrics": {
			input: testutil.GetJson(t, filepath.Join("testdata", "mixed_metrics.json")),
			want:  testutil.GetConf(t, filepath.Join("testdata", "mixed_metrics.yaml")),
		},
		"WithInstanceStorePrefixedMetrics": {
			input: testutil.GetJson(t, filepath.Join("testdata", "instance_store_and_ebs_metrics.json")),
			want:  testutil.GetConf(t, filepath.Join("testdata", "instance_store_and_ebs_metrics.yaml")),
		},
		"WithInstanceStoreOnlyMetrics": {
			input: testutil.GetJson(t, filepath.Join("testdata", "instance_store_only_metrics.json")),
			want:  testutil.GetConf(t, filepath.Join("testdata", "instance_store_only_metrics.yaml")),
		},
		"WithInstanceStoreNonPrefixedMetrics": {
			input: testutil.GetJson(t, filepath.Join("testdata", "instance_store_non_prefixed_metrics.json")),
			want:  testutil.GetConf(t, filepath.Join("testdata", "instance_store_non_prefixed_metrics.yaml")),
		},
	}
	factory := awsnvmereceiver.NewFactory()
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*awsnvmereceiver.Config)
				require.True(t, ok)
				wantCfg := factory.CreateDefaultConfig().(*awsnvmereceiver.Config)
				require.NoError(t, testCase.want.Unmarshal(wantCfg))

				// Some fields are unexported (e.g. enabledSetByUser), so a direct
				// equality check will not work.
				compareConfigsIgnoringEnabledSetByUser(t, wantCfg, gotCfg)
			}
		})
	}
}

// compareConfigsIgnoringEnabledSetByUser compares two configs but ignores the enabledSetByUser field
func compareConfigsIgnoringEnabledSetByUser(t *testing.T, want, got *awsnvmereceiver.Config) {
	// Compare collection interval
	assert.Equal(t, want.CollectionInterval, got.CollectionInterval)

	// Compare resources
	assert.ElementsMatch(t, want.Devices, got.Devices)

	// Compare metrics enabled state for EBS
	assert.Equal(t, want.MetricsBuilderConfig.Metrics.DiskioEbsEc2InstancePerformanceExceededIops.Enabled,
		got.MetricsBuilderConfig.Metrics.DiskioEbsEc2InstancePerformanceExceededIops.Enabled)
	assert.Equal(t, want.MetricsBuilderConfig.Metrics.DiskioEbsEc2InstancePerformanceExceededTp.Enabled,
		got.MetricsBuilderConfig.Metrics.DiskioEbsEc2InstancePerformanceExceededTp.Enabled)
	assert.Equal(t, want.MetricsBuilderConfig.Metrics.DiskioEbsTotalReadBytes.Enabled,
		got.MetricsBuilderConfig.Metrics.DiskioEbsTotalReadBytes.Enabled)
	assert.Equal(t, want.MetricsBuilderConfig.Metrics.DiskioEbsTotalReadOps.Enabled,
		got.MetricsBuilderConfig.Metrics.DiskioEbsTotalReadOps.Enabled)
	assert.Equal(t, want.MetricsBuilderConfig.Metrics.DiskioEbsTotalReadTime.Enabled,
		got.MetricsBuilderConfig.Metrics.DiskioEbsTotalReadTime.Enabled)
	assert.Equal(t, want.MetricsBuilderConfig.Metrics.DiskioEbsTotalWriteBytes.Enabled,
		got.MetricsBuilderConfig.Metrics.DiskioEbsTotalWriteBytes.Enabled)
	assert.Equal(t, want.MetricsBuilderConfig.Metrics.DiskioEbsTotalWriteOps.Enabled,
		got.MetricsBuilderConfig.Metrics.DiskioEbsTotalWriteOps.Enabled)
	assert.Equal(t, want.MetricsBuilderConfig.Metrics.DiskioEbsTotalWriteTime.Enabled,
		got.MetricsBuilderConfig.Metrics.DiskioEbsTotalWriteTime.Enabled)
	assert.Equal(t, want.MetricsBuilderConfig.Metrics.DiskioEbsVolumePerformanceExceededIops.Enabled,
		got.MetricsBuilderConfig.Metrics.DiskioEbsVolumePerformanceExceededIops.Enabled)
	assert.Equal(t, want.MetricsBuilderConfig.Metrics.DiskioEbsVolumePerformanceExceededTp.Enabled,
		got.MetricsBuilderConfig.Metrics.DiskioEbsVolumePerformanceExceededTp.Enabled)
	assert.Equal(t, want.MetricsBuilderConfig.Metrics.DiskioEbsVolumeQueueLength.Enabled,
		got.MetricsBuilderConfig.Metrics.DiskioEbsVolumeQueueLength.Enabled)

	// Compare metrics enabled state for Instance Store
	assert.Equal(t, want.MetricsBuilderConfig.Metrics.DiskioInstanceStorePerformanceExceededIops.Enabled,
		got.MetricsBuilderConfig.Metrics.DiskioInstanceStorePerformanceExceededIops.Enabled)
	assert.Equal(t, want.MetricsBuilderConfig.Metrics.DiskioInstanceStorePerformanceExceededTp.Enabled,
		got.MetricsBuilderConfig.Metrics.DiskioInstanceStorePerformanceExceededTp.Enabled)
	assert.Equal(t, want.MetricsBuilderConfig.Metrics.DiskioInstanceStoreTotalReadBytes.Enabled,
		got.MetricsBuilderConfig.Metrics.DiskioInstanceStoreTotalReadBytes.Enabled)
	assert.Equal(t, want.MetricsBuilderConfig.Metrics.DiskioInstanceStoreTotalReadOps.Enabled,
		got.MetricsBuilderConfig.Metrics.DiskioInstanceStoreTotalReadOps.Enabled)
	assert.Equal(t, want.MetricsBuilderConfig.Metrics.DiskioInstanceStoreTotalReadTime.Enabled,
		got.MetricsBuilderConfig.Metrics.DiskioInstanceStoreTotalReadTime.Enabled)
	assert.Equal(t, want.MetricsBuilderConfig.Metrics.DiskioInstanceStoreTotalWriteBytes.Enabled,
		got.MetricsBuilderConfig.Metrics.DiskioInstanceStoreTotalWriteBytes.Enabled)
	assert.Equal(t, want.MetricsBuilderConfig.Metrics.DiskioInstanceStoreTotalWriteOps.Enabled,
		got.MetricsBuilderConfig.Metrics.DiskioInstanceStoreTotalWriteOps.Enabled)
	assert.Equal(t, want.MetricsBuilderConfig.Metrics.DiskioInstanceStoreTotalWriteTime.Enabled,
		got.MetricsBuilderConfig.Metrics.DiskioInstanceStoreTotalWriteTime.Enabled)
	assert.Equal(t, want.MetricsBuilderConfig.Metrics.DiskioInstanceStoreVolumeQueueLength.Enabled,
		got.MetricsBuilderConfig.Metrics.DiskioInstanceStoreVolumeQueueLength.Enabled)

	// Compare resource attributes
	assert.Equal(t, want.MetricsBuilderConfig.ResourceAttributes.VolumeID.Enabled,
		got.MetricsBuilderConfig.ResourceAttributes.VolumeID.Enabled)
	assert.Equal(t, want.MetricsBuilderConfig.ResourceAttributes.SerialID.Enabled,
		got.MetricsBuilderConfig.ResourceAttributes.SerialID.Enabled)
}

func TestNewTranslator(t *testing.T) {
	// Test with default options
	translator := NewTranslator()
	assert.Equal(t, "awsnvmereceiver", translator.ID().String())

	// Test with custom name
	customName := "custom_name"
	translator = NewTranslator(common.WithName(customName))
	assert.Equal(t, "awsnvmereceiver/"+customName, translator.ID().String())
}
