// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"os"
	"testing"

	"gotest.tools/v3/assert"
)

func TestConfigPathUtil(t *testing.T) {
	tests := []struct {
		name         string
		sectionKey   string
		fileName     string
		defaultValue string
		config       map[string]any
		expectedPath string
		setEnv       bool
		wantError    bool
	}{
		{
			name:       "basic",
			sectionKey: "path",
			config: map[string]any{
				"path": "/prometheus.yaml",
			},
			expectedPath: "/prometheus.yaml",
		},
		{
			name:         "default",
			sectionKey:   "path",
			defaultValue: "/prometheus.yaml",
			config: map[string]any{
				"key": "/other.yaml",
			},
			expectedPath: "/prometheus.yaml",
		},
		{
			name:         "missingInput",
			sectionKey:   "path",
			config:       nil,
			expectedPath: "",
		},
		{
			name:       "file",
			sectionKey: "path",
			config: map[string]any{
				"path": "file:/prometheus.yaml",
			},
			expectedPath: "/prometheus.yaml",
		},
		// skipping ENV due to hardcoded download path
		//{
		//	name:       "env",
		//	sectionKey: "path",
		//	fileName:   "prometheus.yaml",
		//	config: map[string]any{
		//		"path": "env:TEST_ENV",
		//	},
		//	setEnv:       true,
		//	expectedPath: "/opt/aws/amazon-cloudwatch-agent/etc/prometheus.yaml",
		//},
		{
			name:       "missingEnv",
			sectionKey: "path",
			fileName:   "prometheus.yaml",
			config: map[string]any{
				"path": "env:TEST_ENV",
			},
			wantError: true,
		},
	}
	for _, tt := range tests {
		func() {
			t.Run(tt.name, func(t *testing.T) {
				if tt.setEnv {
					defer os.Setenv("TEST_ENV", "")
				}
				got, err := GetConfigPath(tt.fileName, tt.sectionKey, tt.defaultValue, tt.config)
				assert.Equal(t, tt.wantError, err != nil)
				assert.Equal(t, got, tt.expectedPath)
			})
		}()
	}
}
