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
		wantErr error
	}{
		"WithMissingKey": {
			input: map[string]interface{}{"logs": map[string]interface{}{}},
			wantErr: &common.MissingKeyError{
				ID:      tt.ID(),
				JsonKey: baseKey,
			},
		},
		"WithCompleteConfig": {
			input: testutil.GetJson(t, filepath.Join("testdata", "config.json")),
		},
		"WithAllResources": {
			input: testutil.GetJson(t, filepath.Join("testdata", "all_resources.json")),
		},
		"WithSpecificDevices": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"diskio": map[string]interface{}{
							"measurement": []interface{}{
								"diskio_ebs_total_read_ops",
								"diskio_instance_store_total_read_ops",
							},
							"resources": []interface{}{
								"/dev/nvme0n1",
								"/dev/nvme1n1",
							},
						},
					},
				},
			},
		},
		"WithMinimalConfig": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"diskio": map[string]interface{}{
							"measurement": []interface{}{
								"diskio_ebs_total_read_ops",
							},
						},
					},
				},
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*awsnvmereceiver.Config)
				require.True(t, ok)

				// Basic validation - ensure config was created properly
				assert.NotNil(t, gotCfg.Devices)
				assert.NotZero(t, gotCfg.CollectionInterval)
			}
		})
	}
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

func TestGetEnabledMeasurements(t *testing.T) {
	testCases := map[string]struct {
		input    map[string]interface{}
		expected map[string]any
	}{
		"EBSMetrics": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"diskio": map[string]interface{}{
							"measurement": []interface{}{
								"diskio_ebs_total_read_ops",
								"diskio_ebs_total_write_ops",
							},
						},
					},
				},
			},
			expected: map[string]any{
				"diskio_ebs_total_read_ops": map[string]any{
					"enabled": true,
				},
				"diskio_ebs_total_write_ops": map[string]any{
					"enabled": true,
				},
			},
		},
		"InstanceStoreMetrics": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"diskio": map[string]interface{}{
							"measurement": []interface{}{
								"diskio_instance_store_total_read_ops",
								"diskio_instance_store_total_write_ops",
							},
						},
					},
				},
			},
			expected: map[string]any{
				"diskio_instance_store_total_read_ops": map[string]any{
					"enabled": true,
				},
				"diskio_instance_store_total_write_ops": map[string]any{
					"enabled": true,
				},
			},
		},
		"MixedMetrics": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"diskio": map[string]interface{}{
							"measurement": []interface{}{
								"diskio_ebs_total_read_ops",
								"diskio_instance_store_total_read_ops",
								"some_other_metric", // Should be ignored
							},
						},
					},
				},
			},
			expected: map[string]any{
				"diskio_ebs_total_read_ops": map[string]any{
					"enabled": true,
				},
				"diskio_instance_store_total_read_ops": map[string]any{
					"enabled": true,
				},
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			result := getEnabledMeasurements(conf)
			assert.Equal(t, testCase.expected, result)
		})
	}
}
