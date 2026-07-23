// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package containerinsights

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
)

func TestNewTranslators_MissingKey(t *testing.T) {
	// nil config - should return 0 translators (no container_insights key present)
	assert.Equal(t, 0, NewTranslators(nil).Len())
	// empty config - should return 0 translators
	assert.Equal(t, 0, NewTranslators(confmap.NewFromStringMap(map[string]interface{}{})).Len())
}

func TestNewTranslators_RoleNode(t *testing.T) {
	cfg := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"cluster_name": "test-cluster",
			"collect": map[string]interface{}{
				"container_insights": map[string]interface{}{
					"role": "node",
				},
			},
		},
	})
	translators := NewTranslators(cfg)
	// node role: kubeletstats, cadvisor, node_exporter, dcgm, neuron, efa, ebs_csi, lis_csi = 8 pipelines
	assert.Equal(t, 8, translators.Len())
}

func TestNewTranslators_RoleNodeWithLogs(t *testing.T) {
	cfg := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"cluster_name": "test-cluster",
			"collect": map[string]interface{}{
				"container_insights": map[string]interface{}{
					"role": "node",
					"logs": map[string]interface{}{
						"enabled": true,
					},
				},
			},
		},
	})
	translators := NewTranslators(cfg)
	// node role + logs: 8 metric pipelines + 2 log pipelines = 10
	assert.Equal(t, 10, translators.Len())
}

func TestNewTranslators_RoleCluster(t *testing.T) {
	cfg := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"cluster_name": "test-cluster",
			"collect": map[string]interface{}{
				"container_insights": map[string]interface{}{
					"role": "cluster",
				},
			},
		},
	})
	translators := NewTranslators(cfg)
	// cluster role: apiserver, kube_state_metrics = 2 pipelines
	assert.Equal(t, 2, translators.Len())
}

func TestNewTranslators_DefaultRole(t *testing.T) {
	// No role specified, no env var - should default to node
	cfg := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"cluster_name": "test-cluster",
			"collect": map[string]interface{}{
				"container_insights": map[string]interface{}{},
			},
		},
	})
	translators := NewTranslators(cfg)
	// defaults to node role: 8 pipelines
	assert.Equal(t, 8, translators.Len())
}

func TestNewTranslators_EnvVarFallback_Node(t *testing.T) {
	// No role in config, CWAGENT_ROLE=NODE
	t.Setenv(envconfig.CWAGENT_ROLE, envconfig.NODE)
	cfg := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"cluster_name": "test-cluster",
			"collect": map[string]interface{}{
				"container_insights": map[string]interface{}{},
			},
		},
	})
	translators := NewTranslators(cfg)
	// env var NODE -> node role: 8 pipelines
	assert.Equal(t, 8, translators.Len())
}

func TestNewTranslators_EnvVarFallback_Leader(t *testing.T) {
	// No role in config, CWAGENT_ROLE=LEADER
	t.Setenv(envconfig.CWAGENT_ROLE, envconfig.LEADER)
	cfg := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"cluster_name": "test-cluster",
			"collect": map[string]interface{}{
				"container_insights": map[string]interface{}{},
			},
		},
	})
	translators := NewTranslators(cfg)
	// env var LEADER -> cluster role: 2 pipelines
	assert.Equal(t, 2, translators.Len())
}

func TestNewTranslators_JSONConfigOverridesEnvVar(t *testing.T) {
	// JSON says cluster, env var says NODE -> JSON wins
	t.Setenv(envconfig.CWAGENT_ROLE, envconfig.NODE)
	cfg := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"cluster_name": "test-cluster",
			"collect": map[string]interface{}{
				"container_insights": map[string]interface{}{
					"role": "cluster",
				},
			},
		},
	})
	translators := NewTranslators(cfg)
	// JSON config wins: cluster role = 2 pipelines
	assert.Equal(t, 2, translators.Len())
}
