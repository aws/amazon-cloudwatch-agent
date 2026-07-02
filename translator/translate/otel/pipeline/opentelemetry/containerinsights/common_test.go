// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package containerinsights

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"
)

func TestEscapeDollarDigit(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"no dollar", "hello world", "hello world"},
		{"dollar at end", "regex$", "regex$"},
		{"dollar letter", "$FOO", "$FOO"},
		{"dollar brace", "${FOO}", "${FOO}"},
		{"bare $1", "replacement: $1", "replacement: $$1"},
		{"bare $0", "$0", "$$0"},
		{"bare $9", "$9", "$$9"},
		{"already escaped $$1", "$$1", "$$$1"},
		{"triple $$$1", "$$$1", "$$$$1"},
		{"consecutive $1$2", "$1$2", "$$1$$2"},
		{"multi-digit $10", "$10", "$$10"},
		{"mixed text", "tag: k8s.label.$1 and $FOO", "tag: k8s.label.$$1 and $FOO"},
		{"dollar dollar no digit", "$$FOO", "$$FOO"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeDollarDigit(tt.in)
			if got != tt.want {
				t.Errorf("escapeDollarDigit(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

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

	t.Setenv(envCWAgentRole, envRoleLeader)
	assert.Equal(t, modeCluster, getMode(cfg))

	t.Setenv(envCWAgentRole, envRoleNode)
	assert.Equal(t, modeNode, getMode(cfg))
}

func TestGetMode_DefaultsToNode(t *testing.T) {
	os.Unsetenv(envCWAgentRole)
	cfg := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"container_insights": map[string]interface{}{},
			},
		},
	})
	assert.Equal(t, modeNode, getMode(cfg))
}

func TestGetMode_JSONOverridesEnv(t *testing.T) {
	t.Setenv(envCWAgentRole, envRoleNode)
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