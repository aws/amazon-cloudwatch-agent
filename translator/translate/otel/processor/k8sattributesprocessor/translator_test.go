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

// TestTranslateWatchReplicaSetIgnoresDeprecatedContainerInsightsKey: the deprecated
// container_insights section is a no-op, so its watch_replicaset no longer stops the
// ReplicaSet informer. Only the collect-level key does.
func TestTranslateWatchReplicaSetIgnoresDeprecatedContainerInsightsKey(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"container_insights": map[string]interface{}{"watch_replicaset": false},
			},
		},
	})
	cfg, err := NewTranslator("otlp").Translate(conf)
	require.NoError(t, err)
	assert.Contains(t, cfg.(*k8sattributesprocessor.Config).Extract.Metadata, "k8s.deployment.name")
}

// TestTranslateWatchReplicaSetCollectLevel: the collect-level key stops the ReplicaSet informer
// (drops k8s.deployment.name) while keeping k8s.replicaset.name. Key absent -> default true.
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
	assert.Len(t, k8sCfg.Extract.Metadata, 11)

	baseCfg, err := NewTranslator("otlp").Translate(confmap.New())
	require.NoError(t, err)
	assert.Contains(t, baseCfg.(*k8sattributesprocessor.Config).Extract.Metadata, "k8s.deployment.name")
}
