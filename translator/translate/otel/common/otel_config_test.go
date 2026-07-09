// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

func TestGetOtelClusterName(t *testing.T) {
	testCases := map[string]struct {
		input  map[string]any
		envVar string
		want   string
	}{
		"FromOtelRootLevel": {
			input: map[string]any{
				"opentelemetry": map[string]any{
					"cluster_name": "my-otel-cluster",
				},
			},
			want: "my-otel-cluster",
		},
		"EnvVarIgnored": {
			input:  map[string]any{},
			envVar: "env-cluster",
			want:   "",
		},
		"EmptyConfig": {
			input: map[string]any{},
			want:  "",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			if tc.envVar != "" {
				os.Setenv("K8S_CLUSTER_NAME", tc.envVar)
				defer os.Unsetenv("K8S_CLUSTER_NAME")
			} else {
				os.Unsetenv("K8S_CLUSTER_NAME")
			}

			conf := confmap.NewFromStringMap(tc.input)
			got := GetOtelClusterName(conf)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestGetCollectionInterval(t *testing.T) {
	testCases := map[string]struct {
		input      map[string]any
		featureKey string
		want       time.Duration
	}{
		"FeatureSpecificCollectionInterval": {
			input: map[string]any{
				"opentelemetry": map[string]any{
					"collect": map[string]any{
						"container_insights": map[string]any{
							"collection_interval": 45,
						},
					},
				},
			},
			featureKey: "opentelemetry::collect::container_insights",
			want:       45 * time.Second,
		},
		"Default30sWhenNotSet": {
			input:      map[string]any{},
			featureKey: "opentelemetry::collect::container_insights",
			want:       30 * time.Second,
		},
		"ZeroValueIgnored": {
			input: map[string]any{
				"opentelemetry": map[string]any{
					"collect": map[string]any{
						"container_insights": map[string]any{
							"collection_interval": 0,
						},
					},
				},
			},
			featureKey: "opentelemetry::collect::container_insights",
			want:       30 * time.Second,
		},
		"NegativeValueIgnored": {
			input: map[string]any{
				"opentelemetry": map[string]any{
					"collect": map[string]any{
						"container_insights": map[string]any{
							"collection_interval": -5,
						},
					},
				},
			},
			featureKey: "opentelemetry::collect::container_insights",
			want:       30 * time.Second,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(tc.input)
			got := GetCollectionInterval(conf, tc.featureKey)
			require.Equal(t, tc.want, got)
		})
	}
}
