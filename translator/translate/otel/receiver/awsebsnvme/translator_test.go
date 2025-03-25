// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsebsnvme

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/receiver/awsebsnvmereceiver"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/testutil"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	tt := NewTranslator()
	assert.EqualValues(t, "awsebsnvmereceiver", tt.ID().String())
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
		"WithNonEbsMetrics": {
			input: testutil.GetJson(t, filepath.Join("testdata", "non_ebs_metrics.json")),
			want:  testutil.GetConf(t, filepath.Join("testdata", "non_ebs_metrics.yaml")),
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
	}
	factory := awsebsnvmereceiver.NewFactory()
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*awsebsnvmereceiver.Config)
				require.True(t, ok)
				wantCfg := factory.CreateDefaultConfig().(*awsebsnvmereceiver.Config)
				require.NoError(t, testCase.want.Unmarshal(wantCfg))

				// Some fields are unexported (e.g. enabledSetByUser), so a direct
				// equality check will not work.
				compareConfigsIgnoringEnabledSetByUser(t, wantCfg, gotCfg)
			}
		})
	}
}

// compareConfigsIgnoringEnabledSetByUser compares two configs but ignores the enabledSetByUser field
func compareConfigsIgnoringEnabledSetByUser(t *testing.T, want, got *awsebsnvmereceiver.Config) {
	// Compare collection interval
	assert.Equal(t, want.CollectionInterval, got.CollectionInterval)

	// Compare resources
	assert.ElementsMatch(t, want.Resources, got.Resources)

	// Compare metrics enabled state
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

	// Compare resource attributes
	assert.Equal(t, want.MetricsBuilderConfig.ResourceAttributes.VolumeID.Enabled,
		got.MetricsBuilderConfig.ResourceAttributes.VolumeID.Enabled)
}

func TestNewTranslator(t *testing.T) {
	// Test with default options
	translator := NewTranslator()
	assert.Equal(t, "awsebsnvmereceiver", translator.ID().String())

	// Test with custom name
	customName := "custom_name"
	translator = NewTranslator(common.WithName(customName))
	assert.Equal(t, "awsebsnvmereceiver/"+customName, translator.ID().String())
}
