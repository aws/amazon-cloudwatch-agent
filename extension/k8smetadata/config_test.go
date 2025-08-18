// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8smetadata

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"
)

func TestUnmarshalDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NoError(t, confmap.New().Unmarshal(cfg))
	assert.Equal(t, factory.CreateDefaultConfig(), cfg)
}

func TestConfigValidate(t *testing.T) {
	testCases := []struct {
		desc        string
		config      Config
		expectedErr string
	}{
		{
			desc:        "empty objects",
			config:      Config{Objects: []string{}},
			expectedErr: "no k8s objects passed in",
		},
		{
			desc:        "invalid object",
			config:      Config{Objects: []string{"pods"}},
			expectedErr: "invalid k8s object: pods. Only 'endpointslices' and 'services' are allowed",
		},
		{
			desc:        "multiple invalid objects",
			config:      Config{Objects: []string{"pods", "deployments"}},
			expectedErr: "invalid k8s object: pods. Only 'endpointslices' and 'services' are allowed; invalid k8s object: deployments. Only 'endpointslices' and 'services' are allowed",
		},
		{
			desc:        "valid single object",
			config:      Config{Objects: []string{"services"}},
			expectedErr: "",
		},
		{
			desc:        "valid multiple objects",
			config:      Config{Objects: []string{"services", "endpointslices"}},
			expectedErr: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr)
			}
		})
	}
}
