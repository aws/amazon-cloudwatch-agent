// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package azuretagger

import (
	"testing"
	"time"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "empty config",
			config:  &Config{},
			wantErr: false,
		},
		{
			name: "valid config with metadata tags",
			config: &Config{
				AzureMetadataTags: []string{"InstanceId", "InstanceType"},
			},
			wantErr: false,
		},
		{
			name: "valid config with instance tags",
			config: &Config{
				AzureInstanceTagKeys: []string{"Environment", "Team"},
			},
			wantErr: false,
		},
		{
			name: "valid config with wildcard",
			config: &Config{
				AzureInstanceTagKeys: []string{"*"},
			},
			wantErr: false,
		},
		{
			name: "valid config with refresh interval",
			config: &Config{
				RefreshTagsInterval:  5 * time.Minute,
				AzureInstanceTagKeys: []string{"*"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSupportedAppendDimensions(t *testing.T) {
	expected := map[string]string{
		"VMScaleSetName":    "${azure:VMScaleSetName}",
		"ImageId":           "${azure:ImageId}",
		"InstanceId":        "${azure:InstanceId}",
		"InstanceType":      "${azure:InstanceType}",
		"ResourceGroupName": "${azure:ResourceGroupName}",
		"SubscriptionId":    "${azure:SubscriptionId}",
	}

	for key, want := range expected {
		got, ok := SupportedAppendDimensions[key]
		if !ok {
			t.Errorf("SupportedAppendDimensions missing key %q", key)
			continue
		}
		if got != want {
			t.Errorf("SupportedAppendDimensions[%q] = %q, want %q", key, got, want)
		}
	}
}
