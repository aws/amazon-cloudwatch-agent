// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package routing

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/internal/mapstructure"
)

func TestTranslatorID(t *testing.T) {
	tr := NewTranslator("test_name")
	assert.Equal(t, "routing/test_name", tr.ID().String())
}

func TestTranslatorTranslate(t *testing.T) {
	pipelineA := pipeline.NewIDWithName(pipeline.SignalMetrics, "pipeline_a")
	pipelineB := pipeline.NewIDWithName(pipeline.SignalMetrics, "pipeline_b")

	tr := NewTranslator("test",
		WithErrorMode(ottl.IgnoreError),
		WithDefaultPipelines(pipelineA),
		WithTable(routingconnector.RoutingTableItem{
			Condition: `resource.attributes["key"] == "value"`,
			Pipelines: []pipeline.ID{pipelineB},
		}),
	)

	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	routingCfg, ok := cfg.(*routingconnector.Config)
	require.True(t, ok)

	assert.Equal(t, ottl.IgnoreError, routingCfg.ErrorMode)
	assert.Equal(t, []pipeline.ID{pipelineA}, routingCfg.DefaultPipelines)
	require.Len(t, routingCfg.Table, 1)
	assert.Equal(t, `resource.attributes["key"] == "value"`, routingCfg.Table[0].Condition)
	assert.Equal(t, []pipeline.ID{pipelineB}, routingCfg.Table[0].Pipelines)
	assert.Equal(t, routingconnector.Move, routingCfg.Table[0].Action)
}

func TestTranslatorDefaultsActionToMove(t *testing.T) {
	pipelineA := pipeline.NewIDWithName(pipeline.SignalMetrics, "a")
	pipelineB := pipeline.NewIDWithName(pipeline.SignalMetrics, "b")
	tr := NewTranslator("test",
		WithDefaultPipelines(pipelineA),
		WithTable(routingconnector.RoutingTableItem{
			Condition: `attributes["k"] == "v"`,
			Pipelines: []pipeline.ID{pipelineB},
		}),
	)
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	routingCfg := cfg.(*routingconnector.Config)
	assert.Equal(t, routingconnector.Move, routingCfg.Table[0].Action)
}

func TestTranslatorPreservesExplicitAction(t *testing.T) {
	pipelineA := pipeline.NewIDWithName(pipeline.SignalMetrics, "a")
	pipelineB := pipeline.NewIDWithName(pipeline.SignalMetrics, "b")
	tr := NewTranslator("test",
		WithDefaultPipelines(pipelineA),
		WithTable(routingconnector.RoutingTableItem{
			Condition: `attributes["k"] == "v"`,
			Pipelines: []pipeline.ID{pipelineB},
			Action:    routingconnector.Copy,
		}),
	)
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	routingCfg := cfg.(*routingconnector.Config)
	assert.Equal(t, routingconnector.Copy, routingCfg.Table[0].Action)
}

// TestTranslatorConfigRoundTrips ensures the generated config survives a marshal
// (CWA's encoder) + unmarshal (connector factory) round trip and validates.
func TestTranslatorConfigRoundTrips(t *testing.T) {
	pipelineA := pipeline.NewIDWithName(pipeline.SignalMetrics, "a")
	pipelineB := pipeline.NewIDWithName(pipeline.SignalMetrics, "b")
	tr := NewTranslator("test",
		WithErrorMode(ottl.IgnoreError),
		WithDefaultPipelines(pipelineA),
		WithTable(routingconnector.RoutingTableItem{
			Condition: `attributes["k"] == "v"`,
			Pipelines: []pipeline.ID{pipelineB},
		}),
	)
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)

	m, err := mapstructure.Marshal(cfg)
	require.NoError(t, err)

	factory := routingconnector.NewFactory()
	decoded := factory.CreateDefaultConfig()
	require.NoError(t, confmap.NewFromStringMap(m).Unmarshal(decoded))
	require.NoError(t, xconfmap.Validate(decoded))
}

func TestTranslatorDefaults(t *testing.T) {
	tr := NewTranslator("minimal")

	cfg, err := tr.Translate(nil)
	require.NoError(t, err)

	routingCfg := cfg.(*routingconnector.Config)
	assert.Empty(t, routingCfg.DefaultPipelines)
	assert.Empty(t, routingCfg.Table)
}
