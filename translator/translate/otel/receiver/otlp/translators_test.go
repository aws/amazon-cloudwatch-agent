// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlp

import (
	"testing"

	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestNewTranslators(t *testing.T) {
	tests := []struct {
		name         string
		conf         *confmap.Conf
		pipelineName string
		configKey    string
		wantLen      int
		wantSignal   pipeline.Signal
	}{
		{
			name:         "logs signal from config key",
			conf:         confmap.NewFromStringMap(map[string]any{"logs": map[string]any{"grpc_endpoint": "localhost:4317"}}),
			pipelineName: "test",
			configKey:    "logs",
			wantLen:      1,
			wantSignal:   pipeline.SignalLogs,
		},
		{
			name:         "traces signal from config key",
			conf:         confmap.NewFromStringMap(map[string]any{"traces": map[string]any{"http_endpoint": "localhost:4318"}}),
			pipelineName: "test",
			configKey:    "traces",
			wantLen:      1,
			wantSignal:   pipeline.SignalTraces,
		},
		{
			name:         "metrics signal default",
			conf:         confmap.NewFromStringMap(map[string]any{"metrics": map[string]any{"grpc_endpoint": "localhost:4317"}}),
			pipelineName: "test",
			configKey:    "metrics",
			wantLen:      1,
			wantSignal:   pipeline.SignalMetrics,
		},
		{
			name:         "array config",
			conf:         confmap.NewFromStringMap(map[string]any{"logs": []any{map[string]any{"grpc_endpoint": "localhost:4317"}, map[string]any{"http_endpoint": "localhost:4318"}}}),
			pipelineName: "test",
			configKey:    "logs",
			wantLen:      2, // 1 grpc + 1 http
			wantSignal:   pipeline.SignalLogs,
		},
		{
			name:         "app signals pipeline with traces",
			conf:         confmap.NewFromStringMap(map[string]any{common.AppSignalsTraces: map[string]any{"grpc_endpoint": "localhost:4317"}}),
			pipelineName: common.AppSignals,
			configKey:    "traces",
			wantLen:      1,
			wantSignal:   pipeline.SignalTraces,
		},
		{
			name:         "app signals pipeline with metrics fallback",
			conf:         confmap.NewFromStringMap(map[string]any{common.AppSignalsMetricsFallback: map[string]any{"http_endpoint": "localhost:4318"}}),
			pipelineName: common.AppSignals,
			configKey:    "metrics",
			wantLen:      1,
			wantSignal:   pipeline.SignalMetrics,
		},
		{
			name:         "empty config",
			conf:         confmap.NewFromStringMap(map[string]any{}),
			pipelineName: "test",
			configKey:    "logs",
			wantLen:      0,
			wantSignal:   pipeline.SignalLogs,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			translators := NewTranslators(tt.conf, tt.pipelineName, tt.configKey)

			if translators.Len() != tt.wantLen {
				t.Errorf("NewTranslators() len = %v, want %v", translators.Len(), tt.wantLen)
			}
		})
	}
}

func TestNewTranslators_SignalDetection(t *testing.T) {
	tests := []struct {
		name      string
		configKey string
		wantLen   int
	}{
		{"logs prefix", "logs", 1},
		{"traces prefix", "traces", 1},
		{"metrics default", "metrics", 1},
		{"unknown defaults to metrics", "unknown", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(map[string]any{
				tt.configKey: map[string]any{"grpc_endpoint": "localhost:4317"},
			})

			translators := NewTranslators(conf, "test", tt.configKey)

			if translators.Len() != tt.wantLen {
				t.Errorf("NewTranslators() len = %v, want %v", translators.Len(), tt.wantLen)
			}
		})
	}
}

func TestNewTranslators_AppSignalsCompatibility(t *testing.T) {
	tests := []struct {
		name        string
		conf        map[string]any
		configKey   string
		expectedKey string
	}{
		{
			name: "traces with primary key",
			conf: map[string]any{
				common.AppSignalsTraces: map[string]any{"grpc_endpoint": "localhost:4317"},
			},
			configKey:   "traces",
			expectedKey: common.AppSignalsTraces,
		},
		{
			name: "traces with fallback key",
			conf: map[string]any{
				common.AppSignalsTracesFallback: map[string]any{"grpc_endpoint": "localhost:4317"},
			},
			configKey:   "traces",
			expectedKey: common.AppSignalsTracesFallback,
		},
		{
			name: "metrics with primary key",
			conf: map[string]any{
				common.AppSignalsMetrics: map[string]any{"http_endpoint": "localhost:4318"},
			},
			configKey:   "metrics",
			expectedKey: common.AppSignalsMetrics,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(tt.conf)
			translators := NewTranslators(conf, common.AppSignals, tt.configKey)

			if translators.Len() == 0 {
				t.Error("Expected translators to be created for app signals config")
			}
		})
	}
}
