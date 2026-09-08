// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package selftelemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig"
)

func agentConf(selfTelemetry map[string]any) *confmap.Conf {
	agent := map[string]any{"region": "us-west-2"}
	if selfTelemetry != nil {
		agent["self_telemetry"] = selfTelemetry
	}
	return confmap.NewFromStringMap(map[string]any{"agent": agent})
}

func TestEnabled(t *testing.T) {
	testCases := map[string]struct {
		input map[string]any
		want  bool
	}{
		"WithoutBlock": {input: nil, want: false},
		"WithEmpty":    {input: map[string]any{}, want: false},
		"WithDisabled": {input: map[string]any{"enabled": false}, want: false},
		"WithEnabled":  {input: map[string]any{"enabled": true}, want: true},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, testCase.want, Enabled(agentConf(testCase.input)))
		})
	}
}

// TestEnabledIgnoresRootSection pins that the block is read from under agent, so a stale root-level
// block from an earlier config shape does not silently turn the endpoint on.
func TestEnabledIgnoresRootSection(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"self_telemetry": map[string]any{"enabled": true},
	})
	assert.False(t, Enabled(conf))
}

func TestPort(t *testing.T) {
	assert.Equal(t, DefaultPort, Port(agentConf(map[string]any{"enabled": true})))
	assert.Equal(t, 28889, Port(agentConf(map[string]any{"enabled": true, "port": 28889})))
	assert.Equal(t, DefaultPort, Port(agentConf(map[string]any{"enabled": true, "port": 0})))
}

func TestTranslate(t *testing.T) {
	translator := NewTranslator()
	assert.Equal(t, "selftelemetry", translator.ID().String())
	cfg, err := translator.Translate(confmap.New())
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
}

// TestSectionSurvivesJSONMerge is why the block lives under agent: the agent merge rule copies
// sub-keys it has no rule for, so no dedicated merge rule is needed. A root-level section without
// one is dropped before translation, which is the failure this guards against.
func TestSectionSurvivesJSONMerge(t *testing.T) {
	source := map[string]any{
		"agent": map[string]any{
			"region":         "us-west-2",
			"self_telemetry": map[string]any{"enabled": true, "port": 28889},
		},
		// A root section with no rule is dropped, which is what nesting under agent avoids.
		"not_a_registered_section": map[string]any{"enabled": true},
	}
	result := map[string]any{}
	jsonconfig.Merge(source, result)

	require.NotContains(t, result, "not_a_registered_section")
	require.Contains(t, result, "agent")
	assert.Equal(t, source["agent"].(map[string]any)["self_telemetry"],
		result["agent"].(map[string]any)["self_telemetry"])

	conf := confmap.NewFromStringMap(result)
	assert.True(t, Enabled(conf))
	assert.Equal(t, 28889, Port(conf))
}
