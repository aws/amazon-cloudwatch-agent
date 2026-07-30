// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package containerinsights

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
)

func TestGetRole_JSONConfig(t *testing.T) {
	cfg := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"container_insights": map[string]interface{}{
					"role": "cluster",
				},
			},
		},
	})
	assert.Equal(t, roleCluster, getRole(cfg))
}

func TestGetRole_EnvVarFallback(t *testing.T) {
	cfg := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"container_insights": map[string]interface{}{},
			},
		},
	})

	t.Setenv(envconfig.CWAGENT_ROLE, envconfig.LEADER)
	assert.Equal(t, roleCluster, getRole(cfg))

	t.Setenv(envconfig.CWAGENT_ROLE, envconfig.NODE)
	assert.Equal(t, roleNode, getRole(cfg))
}

func TestGetRole_DefaultsToNode(t *testing.T) {
	cfg := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"container_insights": map[string]interface{}{},
			},
		},
	})
	assert.Equal(t, roleNode, getRole(cfg))
}

func TestGetRole_EnvVarCaseInsensitive(t *testing.T) {
	cfg := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"container_insights": map[string]interface{}{},
			},
		},
	})

	t.Setenv(envconfig.CWAGENT_ROLE, "leader") // lowercase
	assert.Equal(t, roleCluster, getRole(cfg))

	t.Setenv(envconfig.CWAGENT_ROLE, "node") // lowercase
	assert.Equal(t, roleNode, getRole(cfg))

	t.Setenv(envconfig.CWAGENT_ROLE, "Leader") // mixed case
	assert.Equal(t, roleCluster, getRole(cfg))
}

func TestGetRole_JSONOverridesEnv(t *testing.T) {
	t.Setenv(envconfig.CWAGENT_ROLE, envconfig.NODE)
	cfg := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"container_insights": map[string]interface{}{
					"role": "cluster",
				},
			},
		},
	})
	assert.Equal(t, roleCluster, getRole(cfg))
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
			assert.Equal(t, tt.want, LogsEnabled(tt.cfg))
		})
	}
}

func TestHasLogPipelines(t *testing.T) {
	// ciConf builds a config with the given container_insights role and logs.enabled.
	// An empty role omits the JSON field so getRole falls back to its default (node).
	ciConf := func(role string, logsEnabled bool) *confmap.Conf {
		ci := map[string]interface{}{
			"logs": map[string]interface{}{"enabled": logsEnabled},
		}
		if role != "" {
			ci["role"] = role
		}
		return confmap.NewFromStringMap(map[string]interface{}{
			"opentelemetry": map[string]interface{}{
				"collect": map[string]interface{}{
					"container_insights": ci,
				},
			},
		})
	}

	tests := []struct {
		name string
		cfg  *confmap.Conf
		want bool
	}{
		{"nil config", nil, false},
		{"container_insights not set", confmap.NewFromStringMap(map[string]interface{}{
			"opentelemetry": map[string]interface{}{"collect": map[string]interface{}{}},
		}), false},
		{"node role with logs enabled", ciConf(roleNode, true), true},
		{"node role with logs disabled", ciConf(roleNode, false), false},
		{"default role (node) with logs enabled", ciConf("", true), true},
		{"cluster role with logs enabled", ciConf(roleCluster, true), false},
		{"cluster role with logs disabled", ciConf(roleCluster, false), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, HasLogPipelines(tt.cfg))
		})
	}
}
