// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtelemetry"

	selftelemetryextension "github.com/aws/amazon-cloudwatch-agent/extension/selftelemetry"
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline"
)

func selfTelemetryID() component.ID {
	return component.NewID(selftelemetryextension.TypeStr)
}

func selfTelemetryInput(selfTelemetry map[string]any) map[string]any {
	agentSection := map[string]any{"region": "us-west-2"}
	if selfTelemetry != nil {
		agentSection["self_telemetry"] = selfTelemetry
	}
	return map[string]any{
		"agent": agentSection,
		"logs": map[string]any{
			"metrics_collected": map[string]any{
				"kubernetes": map[string]any{"cluster_name": "TestCluster"},
			},
		},
	}
}

// TestSelfTelemetryOffByDefault pins the existing behaviour: without the block the collector reports
// nothing and binds no port.
func TestSelfTelemetryOffByDefault(t *testing.T) {
	agent.Global_Config.Region = "us-west-2"
	translator.SetTargetPlatform("linux")

	got, err := TranslateWithoutValidation(selfTelemetryInput(nil), "linux")
	require.NoError(t, err)
	assert.Equal(t, configtelemetry.LevelNone, got.Service.Telemetry.Metrics.Level)
	assert.Empty(t, got.Service.Telemetry.Metrics.Readers)
	assert.NotContains(t, got.Service.Extensions, selfTelemetryID())
}

// TestSelfTelemetryEnabledAddsReaderAndExtension is the whole contract of the config key: one option
// turns on the collector's own metrics and the bridge that puts the scrape registries on them.
func TestSelfTelemetryEnabledAddsReaderAndExtension(t *testing.T) {
	agent.Global_Config.Region = "us-west-2"
	translator.SetTargetPlatform("linux")

	got, err := TranslateWithoutValidation(selfTelemetryInput(map[string]any{
		"enabled": true,
		"port":    28888,
	}), "linux")
	require.NoError(t, err)

	assert.Equal(t, configtelemetry.LevelBasic, got.Service.Telemetry.Metrics.Level)
	require.Len(t, got.Service.Telemetry.Metrics.Readers, 1)
	prom := got.Service.Telemetry.Metrics.Readers[0].Pull.Exporter.Prometheus
	require.NotNil(t, prom)
	assert.Equal(t, 28888, *prom.Port)

	// Without these the SDK renames the series and a scraper no longer recognizes them.
	assert.True(t, *prom.WithoutTypeSuffix)
	assert.True(t, *prom.WithoutUnits)
	assert.True(t, *prom.WithoutScopeInfo)

	assert.Contains(t, got.Service.Extensions, selfTelemetryID())
}

// TestSelfTelemetryAlwaysBindsLoopback is the security property of the design: there is no bind
// address option, so enabling it can never publish the endpoint off the host.
func TestSelfTelemetryAlwaysBindsLoopback(t *testing.T) {
	agent.Global_Config.Region = "us-west-2"
	translator.SetTargetPlatform("linux")

	for _, selfTelemetry := range []map[string]any{
		{"enabled": true},
		{"enabled": true, "port": 39001},
		// A stray host key is rejected by the schema and ignored here; loopback still wins.
		{"enabled": true, "host": "0.0.0.0"},
	} {
		got, err := TranslateWithoutValidation(selfTelemetryInput(selfTelemetry), "linux")
		require.NoError(t, err)
		require.Len(t, got.Service.Telemetry.Metrics.Readers, 1)
		assert.Equal(t, "127.0.0.1",
			*got.Service.Telemetry.Metrics.Readers[0].Pull.Exporter.Prometheus.Host)
	}
}

func TestSelfTelemetryDefaultPort(t *testing.T) {
	agent.Global_Config.Region = "us-west-2"
	translator.SetTargetPlatform("linux")

	got, err := TranslateWithoutValidation(selfTelemetryInput(map[string]any{"enabled": true}), "linux")
	require.NoError(t, err)
	require.Len(t, got.Service.Telemetry.Metrics.Readers, 1)
	assert.Equal(t, 8888, *got.Service.Telemetry.Metrics.Readers[0].Pull.Exporter.Prometheus.Port)
}

// TestSelfTelemetryWithoutPipelines covers the cluster-scraper shape, whose JSON config carries only
// the region and gets every pipeline from the supplied OTEL config. Without a placeholder pipeline
// the translation yields nothing at all and the endpoint never binds.
func TestSelfTelemetryWithoutPipelines(t *testing.T) {
	t.Setenv("SYSTEM_METRICS_ENABLED", "false")
	agent.Global_Config.Region = "us-west-2"
	translator.SetTargetPlatform("linux")

	got, err := TranslateWithoutValidation(map[string]any{
		"agent": map[string]any{
			"region":         "us-west-2",
			"self_telemetry": map[string]any{"enabled": true, "port": 28889},
		},
	}, "linux")
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.NotEmpty(t, got.Service.Pipelines, "a pipeline is required or the config is discarded")
	assert.Equal(t, configtelemetry.LevelBasic, got.Service.Telemetry.Metrics.Level)
	require.Len(t, got.Service.Telemetry.Metrics.Readers, 1)
	assert.Equal(t, 28889, *got.Service.Telemetry.Metrics.Readers[0].Pull.Exporter.Prometheus.Port)
	assert.Contains(t, got.Service.Extensions, selfTelemetryID())
}

// TestNoSelfTelemetryWithoutPipelinesStillFails pins that the placeholder is only added for self
// telemetry, so an otherwise empty config is still rejected as it was before.
func TestNoSelfTelemetryWithoutPipelinesStillFails(t *testing.T) {
	t.Setenv("SYSTEM_METRICS_ENABLED", "false")
	agent.Global_Config.Region = "us-west-2"
	translator.SetTargetPlatform("linux")

	_, err := TranslateWithoutValidation(map[string]any{
		"agent": map[string]any{"region": "us-west-2"},
	}, "linux")
	assert.ErrorIs(t, err, pipeline.ErrNoPipelines)
}

// TestSelfTelemetryStampsNodeViaResource pins the native node-identity path: the collector's own
// resource carries NodeName from the env ref, and the prometheus reader promotes it to a constant
// label on every series. This replaces the hand-rolled per-datapoint stamping and also covers the
// otelcol_* series, which the bridge never touched.
func TestSelfTelemetryStampsNodeViaResource(t *testing.T) {
	agent.Global_Config.Region = "us-west-2"
	translator.SetTargetPlatform("linux")

	got, err := TranslateWithoutValidation(selfTelemetryInput(map[string]any{"enabled": true}), "linux")
	require.NoError(t, err)

	require.Contains(t, got.Service.Telemetry.Resource, "NodeName")
	require.NotNil(t, got.Service.Telemetry.Resource["NodeName"])
	assert.Equal(t, "${env:K8S_NODE_NAME}", *got.Service.Telemetry.Resource["NodeName"])

	require.Len(t, got.Service.Telemetry.Metrics.Readers, 1)
	prom := got.Service.Telemetry.Metrics.Readers[0].Pull.Exporter.Prometheus
	require.NotNil(t, prom.WithResourceConstantLabels)
	assert.Equal(t, []string{"NodeName"}, prom.WithResourceConstantLabels.Included)
}

// TestSelfTelemetryOffLeavesNoResource keeps the resource empty when the feature is off, so nothing
// changes for configs that do not opt in.
func TestSelfTelemetryOffLeavesNoResource(t *testing.T) {
	agent.Global_Config.Region = "us-west-2"
	translator.SetTargetPlatform("linux")

	got, err := TranslateWithoutValidation(selfTelemetryInput(nil), "linux")
	require.NoError(t, err)
	assert.NotContains(t, got.Service.Telemetry.Resource, "NodeName")
}
