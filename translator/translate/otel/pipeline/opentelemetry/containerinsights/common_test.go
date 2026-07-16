// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package containerinsights

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
)

func TestGetMode_JSONConfig(t *testing.T) {
	cfg := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"container_insights": map[string]interface{}{
					"mode": "cluster",
				},
			},
		},
	})
	assert.Equal(t, modeCluster, getMode(cfg))
}

func TestGetMode_EnvVarFallback(t *testing.T) {
	cfg := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"container_insights": map[string]interface{}{},
			},
		},
	})

	t.Setenv(envconfig.CWAGENT_ROLE, envconfig.LEADER)
	assert.Equal(t, modeCluster, getMode(cfg))

	t.Setenv(envconfig.CWAGENT_ROLE, envconfig.NODE)
	assert.Equal(t, modeNode, getMode(cfg))
}

func TestGetMode_DefaultsToNode(t *testing.T) {
	cfg := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"container_insights": map[string]interface{}{},
			},
		},
	})
	assert.Equal(t, modeNode, getMode(cfg))
}

func TestGetMode_EnvVarCaseInsensitive(t *testing.T) {
	cfg := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"container_insights": map[string]interface{}{},
			},
		},
	})

	t.Setenv(envconfig.CWAGENT_ROLE, "leader") // lowercase
	assert.Equal(t, modeCluster, getMode(cfg))

	t.Setenv(envconfig.CWAGENT_ROLE, "node") // lowercase
	assert.Equal(t, modeNode, getMode(cfg))

	t.Setenv(envconfig.CWAGENT_ROLE, "Leader") // mixed case
	assert.Equal(t, modeCluster, getMode(cfg))
}

func TestGetMode_JSONOverridesEnv(t *testing.T) {
	t.Setenv(envconfig.CWAGENT_ROLE, envconfig.NODE)
	cfg := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"container_insights": map[string]interface{}{
					"mode": "cluster",
				},
			},
		},
	})
	assert.Equal(t, modeCluster, getMode(cfg))
}

func TestLogsEnabled(t *testing.T) {
	tests := []struct {
		name string
		cfg  *confmap.Conf
		want bool
	}{
		{"nil config", nil, false},
		{"not set", confmap.NewFromStringMap(map[string]interface{}{
			"opentelemetry": map[string]interface{}{"collect": map[string]interface{}{"container_insights": map[string]interface{}{}}},
		}), false},
		{"enabled true", confmap.NewFromStringMap(map[string]interface{}{
			"opentelemetry": map[string]interface{}{"collect": map[string]interface{}{"container_insights": map[string]interface{}{"logs": map[string]interface{}{"enabled": true}}}},
		}), true},
		{"enabled false", confmap.NewFromStringMap(map[string]interface{}{
			"opentelemetry": map[string]interface{}{"collect": map[string]interface{}{"container_insights": map[string]interface{}{"logs": map[string]interface{}{"enabled": false}}}},
		}), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, logsEnabled(tt.cfg))
		})
	}
}
