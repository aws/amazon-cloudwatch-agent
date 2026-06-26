// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package spanmetrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

func TestTranslator_ID(t *testing.T) {
	tr := NewTranslator("opentelemetry")
	assert.Equal(t, "spanmetrics/opentelemetry", tr.ID().String())
}

func TestTranslator_Translate(t *testing.T) {
	tr := NewTranslator("opentelemetry")
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestIsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected bool
	}{
		{
			name:     "enabled",
			input:    map[string]any{"opentelemetry": map[string]any{"collect": map[string]any{"otlp": map[string]any{"derive_metrics_from_traces": true}}}},
			expected: true,
		},
		{
			name:     "disabled",
			input:    map[string]any{"opentelemetry": map[string]any{"collect": map[string]any{"otlp": map[string]any{"derive_metrics_from_traces": false}}}},
			expected: false,
		},
		{
			name:     "absent",
			input:    map[string]any{"opentelemetry": map[string]any{"collect": map[string]any{"otlp": map[string]any{}}}},
			expected: false,
		},
		{
			name:     "no otlp",
			input:    map[string]any{"opentelemetry": map[string]any{"collect": map[string]any{}}},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(tt.input)
			assert.Equal(t, tt.expected, IsEnabled(conf))
		})
	}
}
