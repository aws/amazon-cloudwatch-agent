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

func TestNewTranslators_ModeNode(t *testing.T) {
	cfg := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"container_insights": map[string]interface{}{
					"cluster_name": "test-cluster",
					"role":         "node",
				},
			},
		},
	})
	translators := NewTranslators(cfg)
	// node mode: kubeletstats, cadvisor, node_exporter, dcgm, neuron, efa, ebs_csi, lis_csi = 8 pipelines
	assert.Equal(t, 8, translators.Len())
}

func TestNewTranslators_ModeNodeWithLogs(t *testing.T) {
	cfg := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"container_insights": map[string]interface{}{
					"cluster_name": "test-cluster",
					"role":         "node",
					"logs": map[string]interface{}{
						"enabled": true,
					},
				},
			},
		},
	})
	translators := NewTranslators(cfg)
	// node mode + logs: 8 metric pipelines + 2 log pipelines = 10
	assert.Equal(t, 10, translators.Len())
}

func TestNewTranslators_ModeCluster(t *testing.T) {
	cfg := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"container_insights": map[string]interface{}{
					"cluster_name": "test-cluster",
					"role":         "cluster",
				},
			},
		},
	})
	translators := NewTranslators(cfg)
	// cluster mode: apiserver, kube_state_metrics = 2 pipelines
	assert.Equal(t, 2, translators.Len())
}

func TestNewTranslators_DefaultMode(t *testing.T) {
	// No mode specified, no env var - should default to node
	cfg := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"container_insights": map[string]interface{}{
					"cluster_name": "test-cluster",
				},
			},
		},
	})
	translators := NewTranslators(cfg)
	// defaults to node mode: 8 pipelines
	assert.Equal(t, 8, translators.Len())
}

func TestNewTranslators_EnvVarFallback_Node(t *testing.T) {
	// No mode in config, CWAGENT_ROLE=NODE
	t.Setenv(envconfig.CWAGENT_ROLE, envconfig.NODE)
	cfg := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"container_insights": map[string]interface{}{
					"cluster_name": "test-cluster",
				},
			},
		},
	})
	translators := NewTranslators(cfg)
	// env var NODE -> node mode: 8 pipelines
	assert.Equal(t, 8, translators.Len())
}

func TestNewTranslators_EnvVarFallback_Leader(t *testing.T) {
	// No mode in config, CWAGENT_ROLE=LEADER
	t.Setenv(envconfig.CWAGENT_ROLE, envconfig.LEADER)
	cfg := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"container_insights": map[string]interface{}{
					"cluster_name": "test-cluster",
				},
			},
		},
	})
	translators := NewTranslators(cfg)
	// env var LEADER -> cluster mode: 2 pipelines
	assert.Equal(t, 2, translators.Len())
}

func TestNewTranslators_JSONConfigOverridesEnvVar(t *testing.T) {
	// JSON says cluster, env var says NODE -> JSON wins
	t.Setenv(envconfig.CWAGENT_ROLE, envconfig.NODE)
	cfg := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"container_insights": map[string]interface{}{
					"cluster_name": "test-cluster",
					"role":         "cluster",
				},
			},
		},
	})
	translators := NewTranslators(cfg)
	// JSON config wins: cluster mode = 2 pipelines
	assert.Equal(t, 2, translators.Len())
}
