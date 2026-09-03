// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sattributesprocessor

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

func TestNewTranslator(t *testing.T) {
	tt := NewTranslator("test")
	assert.Equal(t, "k8s_attributes/test", tt.ID().String())
}

func TestTranslate(t *testing.T) {
	tt := NewTranslator("otlp")
	cfg, err := tt.Translate(nil)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	k8sCfg, ok := cfg.(*k8sattributesprocessor.Config)
	require.True(t, ok)

	assert.Equal(t, "serviceAccount", string(k8sCfg.AuthType))
	assert.False(t, k8sCfg.Passthrough)
	assert.Equal(t, "K8S_NODE_NAME", k8sCfg.Filter.NodeFromEnvVar)
	assert.Empty(t, k8sCfg.Exclude.Pods)
	assert.Contains(t, k8sCfg.Extract.Metadata, "k8s.pod.name")
	assert.Contains(t, k8sCfg.Extract.Metadata, "k8s.namespace.name")
	assert.Contains(t, k8sCfg.Extract.Metadata, "k8s.node.name")
	assert.Contains(t, k8sCfg.Extract.Metadata, "k8s.deployment.name")
	assert.Contains(t, k8sCfg.Extract.Metadata, "k8s.pod.start_time")
	assert.Contains(t, k8sCfg.Extract.Metadata, "k8s.container.name")
	assert.Len(t, k8sCfg.Extract.Metadata, 12)
	assert.Len(t, k8sCfg.Extract.Annotations, 4)
	assert.Len(t, k8sCfg.Extract.Labels, 3)
}

func TestTranslateWatchReplicaSetDisabled(t *testing.T) {
	// container_insights.watch_replicaset=false drops k8s.deployment.name (stops the RS
	// informer) while keeping k8s.replicaset.name (pod ownerRef). Key absent -> default true.
	off := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"container_insights": map[string]interface{}{"watch_replicaset": false},
			},
		},
	})
	cfg, err := NewTranslator("otlp").Translate(off)
	require.NoError(t, err)
	k8sCfg := cfg.(*k8sattributesprocessor.Config)
	assert.NotContains(t, k8sCfg.Extract.Metadata, "k8s.deployment.name")
	assert.Contains(t, k8sCfg.Extract.Metadata, "k8s.replicaset.name")
	assert.Len(t, k8sCfg.Extract.Metadata, 11)

	// Default (key absent) keeps deployment.name.
	baseCfg, err := NewTranslator("otlp").Translate(confmap.New())
	require.NoError(t, err)
	assert.Contains(t, baseCfg.(*k8sattributesprocessor.Config).Extract.Metadata, "k8s.deployment.name")

	// Explicit watch_replicaset=true keeps deployment.name.
	on := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"container_insights": map[string]interface{}{"watch_replicaset": true},
			},
		},
	})
	onCfg, err := NewTranslator("otlp").Translate(on)
	require.NoError(t, err)
	assert.Contains(t, onCfg.(*k8sattributesprocessor.Config).Extract.Metadata, "k8s.deployment.name")
}

// TestTranslateWatchReplicaSetCollectLevel is the decoupling contract: the collect-level key stops the
// ReplicaSet informer with no container_insights key (which would otherwise activate CI on bare presence).
func TestTranslateWatchReplicaSetCollectLevel(t *testing.T) {
	off := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"watch_replicaset": false,
			},
		},
	})
	cfg, err := NewTranslator("otlp").Translate(off)
	require.NoError(t, err)
	k8sCfg := cfg.(*k8sattributesprocessor.Config)
	assert.NotContains(t, k8sCfg.Extract.Metadata, "k8s.deployment.name")
	assert.Contains(t, k8sCfg.Extract.Metadata, "k8s.replicaset.name")
	// No container_insights key was set, so the CI pipelines are never activated by this toggle.
	assert.NotContains(t, off.AllKeys(), "opentelemetry::collect::container_insights")
}
