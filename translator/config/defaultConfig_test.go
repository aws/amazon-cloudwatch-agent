// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultJSONConfigFor_Otel(t *testing.T) {
	cfg, ok := DefaultJSONConfigFor("otel", false, false)
	require.True(t, ok)
	assert.JSONEq(t, defaultOtelConfig, cfg)
}

func TestDefaultJSONConfigFor_Unknown(t *testing.T) {
	_, ok := DefaultJSONConfigFor("unknown", false, false)
	assert.False(t, ok)
}

func TestDefaultJSONConfigFor_Empty(t *testing.T) {
	_, ok := DefaultJSONConfigFor("", false, false)
	assert.False(t, ok)
}

func TestDefaultJSONConfigFor_PlatformVariantsNotAddressableByName(t *testing.T) {
	// The variant suffixes must not be resolvable as base config names, even
	// when a platform is detected (e.g. -c default:otel_ecs on any host). The
	// variants are only reachable via the platform flags on the base name.
	_, ok := DefaultJSONConfigFor("otel_ecs", false, true)
	assert.False(t, ok)
	_, ok = DefaultJSONConfigFor("otel_k8s", true, false)
	assert.False(t, ok)
}

func TestDefaultJSONConfigFor_OtelK8s(t *testing.T) {
	cfg, ok := DefaultJSONConfigFor("otel", true, false)
	require.True(t, ok)
	assert.JSONEq(t, defaultOtelK8sConfig, cfg)
	assert.Contains(t, cfg, "container_insights")
	assert.NotContains(t, cfg, "host_metrics")
}

func TestDefaultJSONConfigFor_OtelECS(t *testing.T) {
	cfg, ok := DefaultJSONConfigFor("otel", false, true)
	require.True(t, ok)
	assert.JSONEq(t, defaultOtelECSConfig, cfg)
	assert.NotContains(t, cfg, "host_metrics")
	assert.NotContains(t, cfg, "container_insights")
}

func TestDefaultJSONConfigFor_KubernetesTakesPrecedenceOverECS(t *testing.T) {
	cfg, ok := DefaultJSONConfigFor("otel", true, true)
	require.True(t, ok)
	assert.JSONEq(t, defaultOtelK8sConfig, cfg)
}

func TestDefaultJSONConfigFor_VMFallsBackToBase(t *testing.T) {
	cfg, ok := DefaultJSONConfigFor("otel", false, false)
	require.True(t, ok)
	assert.JSONEq(t, defaultOtelConfig, cfg)
}

func TestDefaultJSONConfigFor_UnknownWithPlatform(t *testing.T) {
	_, ok := DefaultJSONConfigFor("unknown", true, false)
	assert.False(t, ok)
}
