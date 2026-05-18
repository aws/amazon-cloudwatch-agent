// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsemf

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

func TestSetMetricDescriptors(t *testing.T) {
	tests := []struct {
		name                      string
		config                    map[string]interface{}
		metricUnitKey             string
		expectedMetricDescriptors int
	}{
		{
			name: "WithMetricUnits",
			config: map[string]interface{}{
				"emf_processor": map[string]interface{}{
					"metric_unit": map[string]interface{}{
						"request_duration": "Milliseconds",
						"request_count":    "Count",
					},
				},
			},
			metricUnitKey:             "emf_processor::metric_unit",
			expectedMetricDescriptors: 2,
		},
		{
			name:                      "NoMetricUnits",
			config:                    map[string]interface{}{},
			metricUnitKey:             "emf_processor::metric_unit",
			expectedMetricDescriptors: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(tt.config)
			cfg := &awsemfexporter.Config{}

			err := setMetricDescriptors(conf, tt.metricUnitKey, cfg)
			require.NoError(t, err)
			assert.Len(t, cfg.MetricDescriptors, tt.expectedMetricDescriptors)
		})
	}
}

func TestSetNamespaceWithDefault(t *testing.T) {
	tests := []struct {
		name              string
		config            map[string]interface{}
		namespaceKey      string
		defaultNamespace  string
		expectedNamespace string
	}{
		{
			name: "CustomNamespace",
			config: map[string]interface{}{
				"emf_processor": map[string]interface{}{
					"metric_namespace": "MyApp/Custom",
				},
			},
			namespaceKey:      "emf_processor::metric_namespace",
			defaultNamespace:  "DefaultNamespace",
			expectedNamespace: "MyApp/Custom",
		},
		{
			name:              "UseDefault",
			config:            map[string]interface{}{},
			namespaceKey:      "emf_processor::metric_namespace",
			defaultNamespace:  "DefaultNamespace",
			expectedNamespace: "DefaultNamespace",
		},
		{
			name:              "NoDefaultNoCustom",
			config:            map[string]interface{}{},
			namespaceKey:      "emf_processor::metric_namespace",
			defaultNamespace:  "",
			expectedNamespace: "", // Will use the default from awsemf_default_generic.yaml
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(tt.config)
			cfg := &awsemfexporter.Config{}

			err := setNamespaceWithDefault(conf, tt.namespaceKey, tt.defaultNamespace, cfg)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedNamespace, cfg.Namespace)
		})
	}
}
