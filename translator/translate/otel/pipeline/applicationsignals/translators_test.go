// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package applicationsignals

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestNewTranslatorsTraces(t *testing.T) {
	input := map[string]interface{}{
		"traces": map[string]interface{}{
			"traces_collected": map[string]interface{}{
				"application_signals": map[string]interface{}{},
			},
		},
	}
	conf := confmap.NewFromStringMap(input)
	translators := NewTranslators(conf, pipeline.SignalTraces)
	assert.Equal(t, 1, translators.Len())
}

func TestNewTranslatorsMetrics(t *testing.T) {
	input := map[string]interface{}{
		"logs": map[string]interface{}{
			"metrics_collected": map[string]interface{}{
				"application_signals": map[string]interface{}{},
			},
		},
	}
	conf := confmap.NewFromStringMap(input)
	translators := NewTranslators(conf, pipeline.SignalMetrics)
	assert.Equal(t, 3, translators.Len())
}

func TestNewTranslatorsLogs(t *testing.T) {
	input := map[string]interface{}{
		"logs": map[string]interface{}{
			"metrics_collected": map[string]interface{}{
				"application_signals": map[string]interface{}{},
			},
		},
	}
	conf := confmap.NewFromStringMap(input)
	translators := NewTranslators(conf, pipeline.SignalLogs)
	assert.Equal(t, 3, translators.Len())
}

func TestNewTranslatorsLogsNotEnabled(t *testing.T) {
	input := map[string]interface{}{}
	conf := confmap.NewFromStringMap(input)
	translators := NewTranslators(conf, pipeline.SignalLogs)
	// Translators are always registered; Translate returns MissingKeyError when not enabled
	assert.Equal(t, 3, translators.Len())
	translators.Range(func(pt common.PipelineTranslator) {
		result, err := pt.Translate(conf)
		assert.Nil(t, result)
		assert.Error(t, err)
	})
}

func TestNewTranslatorsLogsAutoOptIn(t *testing.T) {
	input := map[string]interface{}{
		"logs": map[string]interface{}{
			"metrics_collected": map[string]interface{}{
				"application_signals": map[string]interface{}{},
			},
		},
	}
	conf := confmap.NewFromStringMap(input)
	translators := NewTranslators(conf, pipeline.SignalLogs)
	assert.Equal(t, 3, translators.Len())
}

func TestTranslatorMetricsReceiveToRoute(t *testing.T) {
	agent.Global_Config.Region = "us-west-2"
	input := map[string]interface{}{
		"logs": map[string]interface{}{
			"metrics_collected": map[string]interface{}{
				"application_signals": map[string]interface{}{},
			},
		},
	}
	conf := confmap.NewFromStringMap(input)
	tt := NewTranslator(pipeline.SignalMetrics, SetVariant(metricsVariantRoute))
	assert.EqualValues(t, "metrics/application_signals_metrics_route", tt.ID().String())

	got, err := tt.Translate(conf)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, []string{"otlp/grpc_0_0_0_0_4315", "otlp/http_0_0_0_0_4316"}, collections.MapSlice(got.Receivers.Keys(), component.ID.String))
	assert.Empty(t, got.Processors.Keys())
	assert.Equal(t, []string{"routing/application_signals_metrics"}, collections.MapSlice(got.Exporters.Keys(), component.ID.String))
	assert.Equal(t, []string{"routing/application_signals_metrics"}, collections.MapSlice(got.Connectors.Keys(), component.ID.String))
}

func TestTranslatorMetricsRouteToOtlp(t *testing.T) {
	agent.Global_Config.Region = "us-west-2"
	input := map[string]interface{}{
		"logs": map[string]interface{}{
			"metrics_collected": map[string]interface{}{
				"application_signals": map[string]interface{}{},
			},
		},
	}
	conf := confmap.NewFromStringMap(input)
	tt := NewTranslator(pipeline.SignalMetrics, SetVariant(metricsVariantOtlpDest))
	assert.EqualValues(t, "metrics/application_signals_metrics_otlp_destination", tt.ID().String())

	got, err := tt.Translate(conf)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, []string{"routing/application_signals_metrics"}, collections.MapSlice(got.Receivers.Keys(), component.ID.String))
	assert.Equal(t, []string{"batch/application_signals_metrics_otlp_destination"}, collections.MapSlice(got.Processors.Keys(), component.ID.String))
	assert.Equal(t, []string{"otlphttp/application_signals_metrics_otlp_destination"}, collections.MapSlice(got.Exporters.Keys(), component.ID.String))
	assert.Contains(t, collections.MapSlice(got.Extensions.Keys(), component.ID.String), "sigv4auth/monitoring")
}

func TestTranslatorLogsReceiveToRoute(t *testing.T) {
	agent.Global_Config.Region = "us-west-2"
	input := map[string]interface{}{
		"logs": map[string]interface{}{
			"logs_collected": map[string]interface{}{
				"application_signals": map[string]interface{}{},
			},
		},
	}
	conf := confmap.NewFromStringMap(input)
	tt := NewTranslator(pipeline.SignalLogs, SetVariant(logsVariantRoute))
	assert.EqualValues(t, "logs/application_signals_logs_route", tt.ID().String())

	got, err := tt.Translate(conf)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, []string{"otlp/grpc_0_0_0_0_4315", "otlp/http_0_0_0_0_4316"}, collections.MapSlice(got.Receivers.Keys(), component.ID.String))
	assert.Equal(t, []string{"routing/application_signals_logs"}, collections.MapSlice(got.Exporters.Keys(), component.ID.String))
	assert.Equal(t, []string{"routing/application_signals_logs"}, collections.MapSlice(got.Connectors.Keys(), component.ID.String))
}

func TestTranslatorLogsRouteToOtlpBatch(t *testing.T) {
	agent.Global_Config.Region = "us-west-2"
	input := map[string]interface{}{
		"logs": map[string]interface{}{
			"logs_collected": map[string]interface{}{
				"application_signals": map[string]interface{}{},
			},
		},
	}
	conf := confmap.NewFromStringMap(input)
	tt := NewTranslator(pipeline.SignalLogs, SetVariant(logsVariantBatch))
	assert.EqualValues(t, "logs/application_signals_logs_batch", tt.ID().String())

	got, err := tt.Translate(conf)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, []string{"routing/application_signals_logs"}, collections.MapSlice(got.Receivers.Keys(), component.ID.String))
	assert.Contains(t, collections.MapSlice(got.Processors.Keys(), component.ID.String), "batch/application_signals_logs")
	assert.Equal(t, []string{"otlphttp/application_signals_logs"}, collections.MapSlice(got.Exporters.Keys(), component.ID.String))
	assert.Contains(t, collections.MapSlice(got.Extensions.Keys(), component.ID.String), "sigv4auth/logs")
	assert.Contains(t, collections.MapSlice(got.Extensions.Keys(), component.ID.String), "headers_setter/application_signals_logs")
}

func TestTranslatorLogsRouteToOtlpNoBatch(t *testing.T) {
	agent.Global_Config.Region = "us-west-2"
	input := map[string]interface{}{
		"logs": map[string]interface{}{
			"logs_collected": map[string]interface{}{
				"application_signals": map[string]interface{}{},
			},
		},
	}
	conf := confmap.NewFromStringMap(input)
	tt := NewTranslator(pipeline.SignalLogs, SetVariant(logsVariantNoBatch))
	assert.EqualValues(t, "logs/application_signals_logs_nobatch", tt.ID().String())

	got, err := tt.Translate(conf)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, []string{"routing/application_signals_logs"}, collections.MapSlice(got.Receivers.Keys(), component.ID.String))
	assert.Empty(t, got.Processors.Keys())
	assert.Equal(t, []string{"otlphttp/application_signals_logs"}, collections.MapSlice(got.Exporters.Keys(), component.ID.String))
	assert.Contains(t, collections.MapSlice(got.Extensions.Keys(), component.ID.String), "sigv4auth/logs")
}

func TestTranslatorLogsReceiveToRouteDynamic(t *testing.T) {
	agent.Global_Config.Region = "us-west-2"
	input := map[string]interface{}{
		"logs": map[string]interface{}{
			"logs_collected": map[string]interface{}{
				"application_signals": map[string]interface{}{},
			},
		},
	}
	conf := confmap.NewFromStringMap(input)
	tt := NewTranslator(pipeline.SignalLogs, SetVariant(logsVariantRoute))

	got, err := tt.Translate(conf)
	require.NoError(t, err)
	require.NotNil(t, got)

	// Default log group has placeholders ({service.name}), so transform + attributestocontext should be present
	processors := collections.MapSlice(got.Processors.Keys(), component.ID.String)
	assert.Contains(t, processors, "transform/application_signals_logs")
	assert.Contains(t, processors, "attributestocontext")
}

func TestNewTranslatorsNilConf(t *testing.T) {
	translators := NewTranslators(nil, pipeline.SignalMetrics)
	// Translators are always registered; Translate returns error when conf is nil
	assert.Equal(t, 3, translators.Len())
	translators.Range(func(pt common.PipelineTranslator) {
		result, err := pt.Translate(nil)
		assert.Nil(t, result)
		assert.Error(t, err)
	})
}

func TestTranslatorLogsReceiveToRouteStatic(t *testing.T) {
	agent.Global_Config.Region = "us-west-2"
	input := map[string]interface{}{
		"logs": map[string]interface{}{
			"logs_collected": map[string]interface{}{
				"application_signals": map[string]interface{}{
					"log_group_name":  "/my/static/group",
					"log_stream_name": "my-stream",
				},
			},
		},
	}
	conf := confmap.NewFromStringMap(input)
	tt := NewTranslator(pipeline.SignalLogs, SetVariant(logsVariantRoute))

	got, err := tt.Translate(conf)
	require.NoError(t, err)
	require.NotNil(t, got)

	processors := collections.MapSlice(got.Processors.Keys(), component.ID.String)
	assert.NotContains(t, processors, "transform/application_signals_logs")
	assert.NotContains(t, processors, "attributestocontext")
}

func TestTranslatorLogsRouteToOtlpBatchStatic(t *testing.T) {
	agent.Global_Config.Region = "us-west-2"
	input := map[string]interface{}{
		"logs": map[string]interface{}{
			"logs_collected": map[string]interface{}{
				"application_signals": map[string]interface{}{
					"log_group_name":  "/my/static/group",
					"log_stream_name": "my-stream",
				},
			},
		},
	}
	conf := confmap.NewFromStringMap(input)
	tt := NewTranslator(pipeline.SignalLogs, SetVariant(logsVariantBatch))

	got, err := tt.Translate(conf)
	require.NoError(t, err)
	require.NotNil(t, got)

	// Static case: batch processor should not have metadata keys
	assert.Equal(t, []string{"batch/application_signals_logs"}, collections.MapSlice(got.Processors.Keys(), component.ID.String))
	// Headers should use Value (static), verified by headers_setter being present
	assert.Contains(t, collections.MapSlice(got.Extensions.Keys(), component.ID.String), "headers_setter/application_signals_logs")
}

func TestNewTranslatorsLogsDisabled(t *testing.T) {
	input := map[string]interface{}{
		"logs": map[string]interface{}{
			"metrics_collected": map[string]interface{}{
				"application_signals": map[string]interface{}{},
			},
			"logs_collected": map[string]interface{}{
				"application_signals": map[string]interface{}{
					"disabled": true,
				},
			},
		},
	}
	conf := confmap.NewFromStringMap(input)
	translators := NewTranslators(conf, pipeline.SignalLogs)
	assert.Equal(t, 0, translators.Len())
}
