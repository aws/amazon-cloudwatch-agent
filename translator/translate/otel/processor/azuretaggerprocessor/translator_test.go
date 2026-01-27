// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package azuretaggerprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/azuretagger"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	atpTranslator := NewTranslator()
	require.EqualValues(t, "azuretagger", atpTranslator.ID().String())

	testCases := map[string]struct {
		input   map[string]interface{}
		want    *azuretagger.Config
		wantErr error
	}{
		"WithoutAppendDimensions": {
			wantErr: &common.MissingKeyError{
				ID:      atpTranslator.ID(),
				JsonKey: AzuretaggerKey,
			},
		},
		"WithInstanceIdOnly": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"append_dimensions": map[string]interface{}{
						"InstanceId": "${azure:InstanceId}",
					},
				},
			},
			want: &azuretagger.Config{
				RefreshTagsInterval:  0 * time.Second,
				AzureMetadataTags:    []string{"InstanceId"},
				AzureInstanceTagKeys: nil,
			},
		},
		"WithMultipleDimensions": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"append_dimensions": map[string]interface{}{
						"InstanceId":        "${azure:InstanceId}",
						"InstanceType":      "${azure:InstanceType}",
						"ResourceGroupName": "${azure:ResourceGroupName}",
					},
				},
			},
			want: &azuretagger.Config{
				RefreshTagsInterval:  0 * time.Second,
				AzureMetadataTags:    []string{"InstanceId", "InstanceType", "ResourceGroupName"},
				AzureInstanceTagKeys: nil,
			},
		},
		"WithVMScaleSetName": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"append_dimensions": map[string]interface{}{
						"InstanceId":     "${azure:InstanceId}",
						"VMScaleSetName": "${azure:VMScaleSetName}",
					},
				},
			},
			want: &azuretagger.Config{
				RefreshTagsInterval:  0 * time.Second,
				AzureMetadataTags:    []string{"InstanceId"},
				AzureInstanceTagKeys: []string{"VMScaleSetName"},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(tc.input)
			got, err := atpTranslator.Translate(conf)
			if tc.wantErr != nil {
				require.Error(t, err)
				require.Equal(t, tc.wantErr, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, got)
				gotCfg, ok := got.(*azuretagger.Config)
				require.True(t, ok)
				require.Equal(t, tc.want.RefreshTagsInterval, gotCfg.RefreshTagsInterval)
				// Check metadata tags (order may vary)
				require.ElementsMatch(t, tc.want.AzureMetadataTags, gotCfg.AzureMetadataTags)
				require.ElementsMatch(t, tc.want.AzureInstanceTagKeys, gotCfg.AzureInstanceTagKeys)
			}
		})
	}
}

func TestNewTranslatorWithName(t *testing.T) {
	translator := NewTranslatorWithName("custom")
	require.Equal(t, "azuretagger/custom", translator.ID().String())
}
