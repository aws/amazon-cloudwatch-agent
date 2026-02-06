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

func TestSetOTLPLogGroup(t *testing.T) {
	tests := []struct {
		name           string
		config         map[string]interface{}
		expectedLogGrp string
	}{
		{
			name: "CustomLogGroup",
			config: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"otlp": map[string]interface{}{
							"log_group_name": "/aws/application/custom",
						},
					},
				},
			},
			expectedLogGrp: "/aws/application/custom",
		},
		{
			name: "NoLogGroupUsesDefault",
			config: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"otlp": map[string]interface{}{},
					},
				},
			},
			expectedLogGrp: "/aws/cwagent", // Default from awsemf_default_generic.yaml
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(tt.config)
			cfg := &awsemfexporter.Config{
				LogGroupName: "/aws/cwagent", // Default
			}

			setOTLPLogGroup(conf, cfg)
			assert.Equal(t, tt.expectedLogGrp, cfg.LogGroupName)
		})
	}
}

func TestSetOTLPNamespace(t *testing.T) {
	tests := []struct {
		name              string
		config            map[string]interface{}
		expectedNamespace string
	}{
		{
			name: "CustomNamespace",
			config: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"otlp": map[string]interface{}{
							"emf_processor": map[string]interface{}{
								"metric_namespace": "MyApp/Custom",
							},
						},
					},
				},
			},
			expectedNamespace: "MyApp/Custom",
		},
		{
			name: "NoNamespaceUsesDefault",
			config: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"otlp": map[string]interface{}{},
					},
				},
			},
			expectedNamespace: "CWAgent", // Default from awsemf_default_generic.yaml
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(tt.config)
			cfg := &awsemfexporter.Config{
				Namespace: "CWAgent", // Default
			}

			err := setOTLPNamespace(conf, cfg)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedNamespace, cfg.Namespace)
		})
	}
}

func TestSetOTLPMetricDescriptors(t *testing.T) {
	tests := []struct {
		name                      string
		config                    map[string]interface{}
		expectedMetricDescriptors int
	}{
		{
			name: "WithMetricUnits",
			config: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"otlp": map[string]interface{}{
							"emf_processor": map[string]interface{}{
								"metric_unit": map[string]interface{}{
									"request_duration": "Milliseconds",
									"request_count":    "Count",
								},
							},
						},
					},
				},
			},
			expectedMetricDescriptors: 2,
		},
		{
			name: "NoMetricUnits",
			config: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"otlp": map[string]interface{}{},
					},
				},
			},
			expectedMetricDescriptors: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(tt.config)
			cfg := &awsemfexporter.Config{}

			err := setOTLPMetricDescriptors(conf, cfg)
			require.NoError(t, err)
			assert.Len(t, cfg.MetricDescriptors, tt.expectedMetricDescriptors)
		})
	}
}

func TestSetOTLPFields(t *testing.T) {
	config := map[string]interface{}{
		"logs": map[string]interface{}{
			"metrics_collected": map[string]interface{}{
				"otlp": map[string]interface{}{
					"log_group_name": "/aws/application/otlp",
					"emf_processor": map[string]interface{}{
						"metric_namespace": "MyApplication/OTLP",
						"metric_unit": map[string]interface{}{
							"request_duration": "Milliseconds",
							"request_count":    "Count",
						},
					},
				},
			},
		},
	}

	conf := confmap.NewFromStringMap(config)
	cfg := &awsemfexporter.Config{
		LogGroupName: "/aws/cwagent",
		Namespace:    "CWAgent",
	}

	err := setOTLPFields(conf, cfg)
	require.NoError(t, err)
	assert.Equal(t, "/aws/application/otlp", cfg.LogGroupName)
	assert.Equal(t, "MyApplication/OTLP", cfg.Namespace)
	assert.Len(t, cfg.MetricDescriptors, 2)
}

func TestIsOTLP(t *testing.T) {
	tests := []struct {
		name         string
		config       map[string]interface{}
		pipelineName string
		expected     bool
	}{
		{
			name: "OTLPConfigured",
			config: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"otlp": map[string]interface{}{
							"grpc_endpoint": "0.0.0.0:4317",
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "OTLPNotConfigured",
			config: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{},
				},
			},
			expected: false,
		},
		{
			name:     "EmptyConfig",
			config:   map[string]interface{}{},
			expected: false,
		},
		{
			name: "OTLPConfiguredButWrongPipeline",
			config: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"otlp": map[string]interface{}{
							"grpc_endpoint": "0.0.0.0:4317",
						},
					},
				},
			},
			pipelineName: "containerinsights",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(tt.config)
			pipelineName := ""
			if tt.pipelineName != "" {
				pipelineName = tt.pipelineName
			}
			result := isOTLP(conf, pipelineName)
			assert.Equal(t, tt.expected, result)
		})
	}
}
