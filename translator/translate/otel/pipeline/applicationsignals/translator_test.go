// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package applicationsignals

import (
	gocontext "context"
	"fmt"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/eksdetector"
)

func TestTranslatorTraces(t *testing.T) {
	type want struct {
		receivers  []string
		processors []string
		exporters  []string
		extensions []string
	}
	tt := newTranslator(pipeline.SignalTraces)
	assert.EqualValues(t, "traces/application_signals", tt.ID().String())
	testCases := map[string]struct {
		input      map[string]interface{}
		want       *want
		wantErr    error
		isEKSCache func() eksdetector.IsEKSCache
	}{
		"WithoutTracesCollectedKey": {
			input:   map[string]interface{}{},
			wantErr: &common.MissingKeyError{ID: tt.ID(), JsonKey: fmt.Sprint(common.AppSignalsTraces)},
		},
		"WithAppSignalsEnabledTracesEKS": {
			input: map[string]interface{}{
				"traces": map[string]interface{}{
					"traces_collected": map[string]interface{}{
						"application_signals": map[string]interface{}{},
					},
				},
			},
			want: &want{
				receivers:  []string{"otlp/grpc_0_0_0_0_4315", "otlp/http_0_0_0_0_4316"},
				processors: []string{"resourcedetection", "awsapplicationsignals"},
				exporters:  []string{"awsxray/application_signals"},
				extensions: []string{"awsproxy/application_signals", "agenthealth/traces", "agenthealth/statuscode"},
			},
			isEKSCache: eksdetector.TestIsEKSCacheEKS,
		},
		"WithAppSignalsEnabledK8s": {
			input: map[string]interface{}{
				"traces": map[string]interface{}{
					"traces_collected": map[string]interface{}{
						"application_signals": map[string]interface{}{},
					},
				},
			},
			want: &want{
				receivers:  []string{"otlp/grpc_0_0_0_0_4315", "otlp/http_0_0_0_0_4316"},
				processors: []string{"resourcedetection", "awsapplicationsignals"},
				exporters:  []string{"awsxray/application_signals"},
				extensions: []string{"awsproxy/application_signals", "agenthealth/traces", "agenthealth/statuscode"},
			},
			isEKSCache: eksdetector.TestIsEKSCacheK8s,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Setenv(common.KubernetesEnvVar, "TEST")
			eksdetector.IsEKS = testCase.isEKSCache
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if testCase.want == nil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, testCase.want.receivers, collections.MapSlice(got.Receivers.Keys(), component.ID.String))
				assert.Equal(t, testCase.want.processors, collections.MapSlice(got.Processors.Keys(), component.ID.String))
				assert.Equal(t, testCase.want.exporters, collections.MapSlice(got.Exporters.Keys(), component.ID.String))
				assert.Equal(t, testCase.want.extensions, collections.MapSlice(got.Extensions.Keys(), component.ID.String))
			}
		})
	}
}

func TestTranslatorMetricsForKubernetes(t *testing.T) {
	type want struct {
		receivers  []string
		processors []string
		exporters  []string
		extensions []string
	}
	tt := newTranslator(pipeline.SignalMetrics, setVariant(metricsVariantLogDest))
	assert.EqualValues(t, "metrics/application_signals_metrics_logs_destination", tt.ID().String())
	testCases := map[string]struct {
		input          map[string]interface{}
		want           *want
		wantErr        error
		isEKSCache     func() eksdetector.IsEKSCache
		kubernetesMode string
	}{
		"WithoutMetricsCollectedKey": {
			input:   map[string]interface{}{},
			wantErr: &common.MissingKeyError{ID: tt.ID(), JsonKey: fmt.Sprint(common.AppSignalsMetrics)},
		},
		"WithAppSignalsEnabledMetrics": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"application_signals": map[string]interface{}{},
					},
				},
			},
			want: &want{
				receivers:  []string{"routing/application_signals_metrics"},
				processors: []string{"metricstransform/application_signals", "resourcedetection", "awsapplicationsignals", "awsentity/service/application_signals"},
				exporters:  []string{"awsemf/application_signals"},
				extensions: []string{"k8smetadata", "agenthealth/logs", "agenthealth/statuscode"},
			},
			isEKSCache:     eksdetector.TestIsEKSCacheEKS,
			kubernetesMode: config.ModeEKS,
		},
		"WithAppSignalsAndLoggingEnabled": {
			input: map[string]interface{}{
				"agent": map[string]interface{}{
					"debug": true,
				},
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"application_signals": map[string]interface{}{},
					},
				},
			},
			want: &want{
				receivers:  []string{"routing/application_signals_metrics"},
				processors: []string{"metricstransform/application_signals", "resourcedetection", "awsapplicationsignals", "awsentity/service/application_signals"},
				exporters:  []string{"debug/application_signals", "awsemf/application_signals"},
				extensions: []string{"k8smetadata", "agenthealth/logs", "agenthealth/statuscode"},
			},
			isEKSCache:     eksdetector.TestIsEKSCacheEKS,
			kubernetesMode: config.ModeEKS,
		},
		"WithAppSignalsEnabledK8s": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"application_signals": map[string]interface{}{},
					},
				},
			},
			want: &want{
				receivers:  []string{"routing/application_signals_metrics"},
				processors: []string{"metricstransform/application_signals", "resourcedetection", "awsapplicationsignals", "awsentity/service/application_signals"},
				exporters:  []string{"awsemf/application_signals"},
				extensions: []string{"k8smetadata", "agenthealth/logs", "agenthealth/statuscode"},
			},
			isEKSCache:     eksdetector.TestIsEKSCacheK8s,
			kubernetesMode: config.ModeEKS,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Setenv(common.KubernetesEnvVar, "TEST")
			eksdetector.IsEKS = testCase.isEKSCache
			context.CurrentContext().SetKubernetesMode(testCase.kubernetesMode)
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if testCase.want == nil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, testCase.want.receivers, collections.MapSlice(got.Receivers.Keys(), component.ID.String))
				assert.Equal(t, testCase.want.processors, collections.MapSlice(got.Processors.Keys(), component.ID.String))
				assert.Equal(t, testCase.want.exporters, collections.MapSlice(got.Exporters.Keys(), component.ID.String))
				assert.Equal(t, testCase.want.extensions, collections.MapSlice(got.Extensions.Keys(), component.ID.String))
			}
		})
	}
}

func TestTranslatorMetricsForEC2(t *testing.T) {
	type want struct {
		receivers  []string
		processors []string
		exporters  []string
		extensions []string
	}
	tt := newTranslator(pipeline.SignalMetrics, setVariant(metricsVariantLogDest))
	assert.EqualValues(t, "metrics/application_signals_metrics_logs_destination", tt.ID().String())
	testCases := map[string]struct {
		input      map[string]interface{}
		want       *want
		wantErr    error
		isEKSCache func() eksdetector.IsEKSCache
	}{
		"WithoutMetricsCollectedKey": {
			input:   map[string]interface{}{},
			wantErr: &common.MissingKeyError{ID: tt.ID(), JsonKey: fmt.Sprint(common.AppSignalsMetrics)},
		},
		"WithAppSignalsEnabledMetrics": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"application_signals": map[string]interface{}{},
					},
				},
			},
			want: &want{
				receivers:  []string{"routing/application_signals_metrics"},
				processors: []string{"metricstransform/application_signals", "resourcedetection", "awsapplicationsignals", "awsentity/service/application_signals"},
				exporters:  []string{"awsemf/application_signals"},
				extensions: []string{"agenthealth/logs", "agenthealth/statuscode"},
			},
			isEKSCache: eksdetector.TestIsEKSCacheEKS,
		},
		"WithAppSignalsAndLoggingEnabled": {
			input: map[string]interface{}{
				"agent": map[string]interface{}{
					"debug": true,
				},
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"application_signals": map[string]interface{}{},
					},
				},
			},
			want: &want{
				receivers:  []string{"routing/application_signals_metrics"},
				processors: []string{"metricstransform/application_signals", "resourcedetection", "awsapplicationsignals", "awsentity/service/application_signals"},
				exporters:  []string{"debug/application_signals", "awsemf/application_signals"},
				extensions: []string{"agenthealth/logs", "agenthealth/statuscode"},
			},
			isEKSCache: eksdetector.TestIsEKSCacheEKS,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx := context.CurrentContext()
			context.CurrentContext().SetKubernetesMode("")
			ctx.SetMode(config.ModeEC2)
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if testCase.want == nil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, testCase.want.receivers, collections.MapSlice(got.Receivers.Keys(), component.ID.String))
				assert.Equal(t, testCase.want.processors, collections.MapSlice(got.Processors.Keys(), component.ID.String))
				assert.Equal(t, testCase.want.exporters, collections.MapSlice(got.Exporters.Keys(), component.ID.String))
				assert.Equal(t, testCase.want.extensions, collections.MapSlice(got.Extensions.Keys(), component.ID.String))
			}
		})
	}
}

// TestTranslatorMetricsForECS tests that the awsentity processor is not added
func TestTranslatorMetricsForECS(t *testing.T) {
	type want struct {
		receivers  []string
		processors []string
		exporters  []string
		extensions []string
	}
	tt := newTranslator(pipeline.SignalMetrics, setVariant(metricsVariantLogDest))
	assert.EqualValues(t, "metrics/application_signals_metrics_logs_destination", tt.ID().String())
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *want
		wantErr error
	}{
		"WithoutMetricsCollectedKey": {
			input:   map[string]interface{}{},
			wantErr: &common.MissingKeyError{ID: tt.ID(), JsonKey: fmt.Sprint(common.AppSignalsMetrics)},
		},
		"WithAppSignalsEnabledMetrics": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"application_signals": map[string]interface{}{},
					},
				},
			},
			want: &want{
				receivers:  []string{"routing/application_signals_metrics"},
				processors: []string{"metricstransform/application_signals", "resourcedetection", "awsapplicationsignals"},
				exporters:  []string{"awsemf/application_signals"},
				extensions: []string{"agenthealth/logs", "agenthealth/statuscode"},
			},
		},
		"WithAppSignalsAndLoggingEnabled": {
			input: map[string]interface{}{
				"agent": map[string]interface{}{
					"debug": true,
				},
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"application_signals": map[string]interface{}{},
					},
				},
			},
			want: &want{
				receivers:  []string{"routing/application_signals_metrics"},
				processors: []string{"metricstransform/application_signals", "resourcedetection", "awsapplicationsignals"},
				exporters:  []string{"debug/application_signals", "awsemf/application_signals"},
				extensions: []string{"agenthealth/logs", "agenthealth/statuscode"},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			context.CurrentContext().SetRunInContainer(true)
			t.Setenv(config.RUN_IN_CONTAINER, config.RUN_IN_CONTAINER_TRUE)
			ecsutil.GetECSUtilSingleton().Region = "test"

			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if testCase.want == nil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, testCase.want.receivers, collections.MapSlice(got.Receivers.Keys(), component.ID.String))
				assert.Equal(t, testCase.want.processors, collections.MapSlice(got.Processors.Keys(), component.ID.String))
				assert.Equal(t, testCase.want.exporters, collections.MapSlice(got.Exporters.Keys(), component.ID.String))
				assert.Equal(t, testCase.want.extensions, collections.MapSlice(got.Extensions.Keys(), component.ID.String))
			}
		})
	}
}

func TestTranslatorLogsRoute(t *testing.T) {
	type want struct {
		receivers  []string
		processors []string
		exporters  []string
		connectors []string
	}
	agent.Global_Config.Region = "us-west-2"
	tt := newTranslator(pipeline.SignalLogs, setVariant(logsVariantRoute))
	assert.EqualValues(t, "logs/application_signals_logs_route", tt.ID().String())
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *want
		wantErr error
	}{
		"WithoutLogsCollectedKey": {
			input:   map[string]interface{}{},
			wantErr: &common.MissingKeyError{ID: tt.ID(), JsonKey: fmt.Sprint(common.AppSignalsLogs)},
		},
		"WithLogsEnabled": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"application_signals": map[string]interface{}{},
					},
				},
			},
			want: &want{
				receivers:  []string{"otlp/grpc_0_0_0_0_4315", "otlp/http_0_0_0_0_4316"},
				processors: []string{"transform/application_signals_logs", "attributestocontext", "transform/application_signals_logs_cleanup"},
				exporters:  []string{"routing/application_signals_logs"},
				connectors: []string{"routing/application_signals_logs"},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if testCase.want == nil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, testCase.want.receivers, collections.MapSlice(got.Receivers.Keys(), component.ID.String))
				assert.Equal(t, testCase.want.processors, collections.MapSlice(got.Processors.Keys(), component.ID.String))
				assert.Equal(t, testCase.want.exporters, collections.MapSlice(got.Exporters.Keys(), component.ID.String))
				assert.Equal(t, testCase.want.connectors, collections.MapSlice(got.Connectors.Keys(), component.ID.String))
			}
		})
	}
}

func TestTranslatorLogsBatch(t *testing.T) {
	agent.Global_Config.Region = "us-west-2"
	tt := newTranslator(pipeline.SignalLogs, setVariant(logsVariantBatch))
	assert.EqualValues(t, "logs/application_signals_logs_batch", tt.ID().String())

	input := map[string]interface{}{
		"logs": map[string]interface{}{
			"logs_collected": map[string]interface{}{
				"application_signals": map[string]interface{}{},
			},
		},
	}
	conf := confmap.NewFromStringMap(input)
	got, err := tt.Translate(conf)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, []string{"routing/application_signals_logs"}, collections.MapSlice(got.Receivers.Keys(), component.ID.String))
	assert.Contains(t, collections.MapSlice(got.Processors.Keys(), component.ID.String), "batch/application_signals_logs")
	assert.Equal(t, []string{"otlphttp/application_signals_logs"}, collections.MapSlice(got.Exporters.Keys(), component.ID.String))
	assert.Contains(t, collections.MapSlice(got.Extensions.Keys(), component.ID.String), "headers_setter/application_signals_logs")
	assert.Contains(t, collections.MapSlice(got.Extensions.Keys(), component.ID.String), "sigv4auth/logs")
	assert.Contains(t, collections.MapSlice(got.Extensions.Keys(), component.ID.String), "awscloudwatchlogsprovisioner")
}

func TestTranslatorLogsNoBatch(t *testing.T) {
	agent.Global_Config.Region = "us-west-2"
	tt := newTranslator(pipeline.SignalLogs, setVariant(logsVariantNoBatch))
	assert.EqualValues(t, "logs/application_signals_logs_nobatch", tt.ID().String())

	input := map[string]interface{}{
		"logs": map[string]interface{}{
			"logs_collected": map[string]interface{}{
				"application_signals": map[string]interface{}{},
			},
		},
	}
	conf := confmap.NewFromStringMap(input)
	got, err := tt.Translate(conf)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, []string{"routing/application_signals_logs"}, collections.MapSlice(got.Receivers.Keys(), component.ID.String))
	assert.Empty(t, got.Processors.Keys())
	assert.Equal(t, []string{"otlphttp/application_signals_logs"}, collections.MapSlice(got.Exporters.Keys(), component.ID.String))
	assert.Contains(t, collections.MapSlice(got.Extensions.Keys(), component.ID.String), "sigv4auth/logs")
}

func TestServiceEndpoint(t *testing.T) {
	tests := []struct {
		service  string
		region   string
		path     string
		expected string
	}{
		// Standard partition
		{"logs", "us-east-1", "/v1/logs", "https://logs.us-east-1.amazonaws.com/v1/logs"},
		{"logs", "eu-west-1", "/v1/logs", "https://logs.eu-west-1.amazonaws.com/v1/logs"},
		{"monitoring", "us-west-2", "/v1/metrics", "https://monitoring.us-west-2.amazonaws.com/v1/metrics"},
		{"monitoring", "ap-southeast-1", "/v1/metrics", "https://monitoring.ap-southeast-1.amazonaws.com/v1/metrics"},
		// China partition
		{"logs", "cn-north-1", "/v1/logs", "https://logs.cn-north-1.amazonaws.com.cn/v1/logs"},
		{"logs", "cn-northwest-1", "/v1/logs", "https://logs.cn-northwest-1.amazonaws.com.cn/v1/logs"},
		{"monitoring", "cn-north-1", "/v1/metrics", "https://monitoring.cn-north-1.amazonaws.com.cn/v1/metrics"},
		{"monitoring", "cn-northwest-1", "/v1/metrics", "https://monitoring.cn-northwest-1.amazonaws.com.cn/v1/metrics"},
		// GovCloud partition
		{"logs", "us-gov-west-1", "/v1/logs", "https://logs.us-gov-west-1.amazonaws.com/v1/logs"},
		{"logs", "us-gov-east-1", "/v1/logs", "https://logs.us-gov-east-1.amazonaws.com/v1/logs"},
		{"monitoring", "us-gov-west-1", "/v1/metrics", "https://monitoring.us-gov-west-1.amazonaws.com/v1/metrics"},
		{"monitoring", "us-gov-east-1", "/v1/metrics", "https://monitoring.us-gov-east-1.amazonaws.com/v1/metrics"},
		// ISO partition
		{"logs", "us-iso-east-1", "/v1/logs", "https://logs.us-iso-east-1.c2s.ic.gov/v1/logs"},
		{"monitoring", "us-iso-east-1", "/v1/metrics", "https://monitoring.us-iso-east-1.c2s.ic.gov/v1/metrics"},
		// ISOB partition
		{"logs", "us-isob-east-1", "/v1/logs", "https://logs.us-isob-east-1.sc2s.sgov.gov/v1/logs"},
		{"monitoring", "us-isob-east-1", "/v1/metrics", "https://monitoring.us-isob-east-1.sc2s.sgov.gov/v1/metrics"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s/%s", tt.service, tt.region), func(t *testing.T) {
			assert.Equal(t, tt.expected, serviceEndpoint(tt.service, tt.region, tt.path))
		})
	}
}

func TestBuildOTTLSetStatementsNilPlaceholder(t *testing.T) {
	statements := buildOTTLSetStatements(metadataKeyLogGroup, parseTemplate("/aws/service-events/{service.name}"))

	factory := transformprocessor.NewFactory()
	cfg := factory.CreateDefaultConfig().(*transformprocessor.Config)
	stmts := make([]interface{}, len(statements))
	for i, s := range statements {
		stmts[i] = s
	}
	cfgMap := map[string]interface{}{
		"log_statements": []interface{}{
			map[string]interface{}{
				"context":    "resource",
				"error_mode": "propagate",
				"statements": stmts,
			},
		},
	}
	require.NoError(t, confmap.NewFromStringMap(cfgMap).Unmarshal(&cfg))

	sink := new(consumertest.LogsSink)
	proc, err := factory.CreateLogs(gocontext.Background(), processortest.NewNopSettings(factory.Type()), cfg, sink)
	require.NoError(t, err)
	require.NoError(t, proc.Start(gocontext.Background(), nil))
	defer proc.Shutdown(gocontext.Background())

	// service.name NOT set — Concat produces "<nil>", replace_pattern fixes it
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	err = proc.ConsumeLogs(gocontext.Background(), logs)
	require.NoError(t, err)

	result := sink.AllLogs()
	require.Equal(t, 1, len(result))
	attrs := result[0].ResourceLogs().At(0).Resource().Attributes()
	val, found := attrs.Get(metadataKeyLogGroup)
	require.True(t, found, "destination attribute should be set")
	assert.Equal(t, "/aws/service-events/unknown_service", val.Str())
}

func TestBuildOTTLSetStatementsResolvedPlaceholder(t *testing.T) {
	statements := buildOTTLSetStatements(metadataKeyLogGroup, parseTemplate("/aws/service-events/{service.name}"))

	factory := transformprocessor.NewFactory()
	cfg := factory.CreateDefaultConfig().(*transformprocessor.Config)
	stmts := make([]interface{}, len(statements))
	for i, s := range statements {
		stmts[i] = s
	}
	cfgMap := map[string]interface{}{
		"log_statements": []interface{}{
			map[string]interface{}{
				"context":    "resource",
				"error_mode": "propagate",
				"statements": stmts,
			},
		},
	}
	require.NoError(t, confmap.NewFromStringMap(cfgMap).Unmarshal(&cfg))

	sink := new(consumertest.LogsSink)
	proc, err := factory.CreateLogs(gocontext.Background(), processortest.NewNopSettings(factory.Type()), cfg, sink)
	require.NoError(t, err)
	require.NoError(t, proc.Start(gocontext.Background(), nil))
	defer proc.Shutdown(gocontext.Background())

	// service.name IS set — Concat resolves normally, replace_pattern is no-op
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "my-service")
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	err = proc.ConsumeLogs(gocontext.Background(), logs)
	require.NoError(t, err)

	result := sink.AllLogs()
	require.Equal(t, 1, len(result))
	attrs := result[0].ResourceLogs().At(0).Resource().Attributes()
	val, found := attrs.Get(metadataKeyLogGroup)
	require.True(t, found)
	assert.Equal(t, "/aws/service-events/my-service", val.Str())
}

func TestBuildOTTLSetStatementsUnknownServiceTruncation(t *testing.T) {
	statements := buildOTTLSetStatements(metadataKeyLogGroup, parseTemplate("/aws/service-events/{service.name}"))

	factory := transformprocessor.NewFactory()
	cfg := factory.CreateDefaultConfig().(*transformprocessor.Config)
	stmts := make([]interface{}, len(statements))
	for i, s := range statements {
		stmts[i] = s
	}
	cfgMap := map[string]interface{}{
		"log_statements": []interface{}{
			map[string]interface{}{
				"context":    "resource",
				"error_mode": "propagate",
				"statements": stmts,
			},
		},
	}
	require.NoError(t, confmap.NewFromStringMap(cfgMap).Unmarshal(&cfg))

	sink := new(consumertest.LogsSink)
	proc, err := factory.CreateLogs(gocontext.Background(), processortest.NewNopSettings(factory.Type()), cfg, sink)
	require.NoError(t, err)
	require.NoError(t, proc.Start(gocontext.Background(), nil))
	defer proc.Shutdown(gocontext.Background())

	// service.name is "unknown_service:java" — should be truncated to "unknown_service"
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "unknown_service:java")
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	err = proc.ConsumeLogs(gocontext.Background(), logs)
	require.NoError(t, err)

	result := sink.AllLogs()
	require.Equal(t, 1, len(result))
	attrs := result[0].ResourceLogs().At(0).Resource().Attributes()
	val, found := attrs.Get(metadataKeyLogGroup)
	require.True(t, found)
	assert.Equal(t, "/aws/service-events/unknown_service", val.Str())
}

func TestBuildOTTLSetStatementsNonServiceNameNil(t *testing.T) {
	statements := buildOTTLSetStatements(metadataKeyLogStream, parseTemplate("{host.name}/{service.instance.id}"))

	factory := transformprocessor.NewFactory()
	cfg := factory.CreateDefaultConfig().(*transformprocessor.Config)
	stmts := make([]interface{}, len(statements))
	for i, s := range statements {
		stmts[i] = s
	}
	cfgMap := map[string]interface{}{
		"log_statements": []interface{}{
			map[string]interface{}{
				"context":    "resource",
				"error_mode": "propagate",
				"statements": stmts,
			},
		},
	}
	require.NoError(t, confmap.NewFromStringMap(cfgMap).Unmarshal(&cfg))

	sink := new(consumertest.LogsSink)
	proc, err := factory.CreateLogs(gocontext.Background(), processortest.NewNopSettings(factory.Type()), cfg, sink)
	require.NoError(t, err)
	require.NoError(t, proc.Start(gocontext.Background(), nil))
	defer proc.Shutdown(gocontext.Background())

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	err = proc.ConsumeLogs(gocontext.Background(), logs)
	require.NoError(t, err)

	result := sink.AllLogs()
	require.Equal(t, 1, len(result))
	attrs := result[0].ResourceLogs().At(0).Resource().Attributes()
	val, found := attrs.Get(metadataKeyLogStream)
	require.True(t, found)
	assert.Equal(t, "unknown/unknown", val.Str())
}

func TestBuildOTTLSetStatementsPartialNil(t *testing.T) {
	statements := buildOTTLSetStatements(metadataKeyLogStream, parseTemplate("{host.name}/{service.instance.id}"))

	factory := transformprocessor.NewFactory()
	cfg := factory.CreateDefaultConfig().(*transformprocessor.Config)
	stmts := make([]interface{}, len(statements))
	for i, s := range statements {
		stmts[i] = s
	}
	cfgMap := map[string]interface{}{
		"log_statements": []interface{}{
			map[string]interface{}{
				"context":    "resource",
				"error_mode": "propagate",
				"statements": stmts,
			},
		},
	}
	require.NoError(t, confmap.NewFromStringMap(cfgMap).Unmarshal(&cfg))

	sink := new(consumertest.LogsSink)
	proc, err := factory.CreateLogs(gocontext.Background(), processortest.NewNopSettings(factory.Type()), cfg, sink)
	require.NoError(t, err)
	require.NoError(t, proc.Start(gocontext.Background(), nil))
	defer proc.Shutdown(gocontext.Background())

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("host.name", "my-host")
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	err = proc.ConsumeLogs(gocontext.Background(), logs)
	require.NoError(t, err)

	result := sink.AllLogs()
	require.Equal(t, 1, len(result))
	attrs := result[0].ResourceLogs().At(0).Resource().Attributes()
	val, found := attrs.Get(metadataKeyLogStream)
	require.True(t, found)
	assert.Equal(t, "my-host/unknown", val.Str())
}

func TestBuildOTTLSetStatementsSourceNotMutated(t *testing.T) {
	statements := buildOTTLSetStatements(metadataKeyLogGroup, parseTemplate("/aws/service-events/{service.name}"))

	factory := transformprocessor.NewFactory()
	cfg := factory.CreateDefaultConfig().(*transformprocessor.Config)
	stmts := make([]interface{}, len(statements))
	for i, s := range statements {
		stmts[i] = s
	}
	cfgMap := map[string]interface{}{
		"log_statements": []interface{}{
			map[string]interface{}{
				"context":    "resource",
				"error_mode": "propagate",
				"statements": stmts,
			},
		},
	}
	require.NoError(t, confmap.NewFromStringMap(cfgMap).Unmarshal(&cfg))

	sink := new(consumertest.LogsSink)
	proc, err := factory.CreateLogs(gocontext.Background(), processortest.NewNopSettings(factory.Type()), cfg, sink)
	require.NoError(t, err)
	require.NoError(t, proc.Start(gocontext.Background(), nil))
	defer proc.Shutdown(gocontext.Background())

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "unknown_service:python")
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	err = proc.ConsumeLogs(gocontext.Background(), logs)
	require.NoError(t, err)

	result := sink.AllLogs()
	require.Equal(t, 1, len(result))
	attrs := result[0].ResourceLogs().At(0).Resource().Attributes()
	srcVal, found := attrs.Get("service.name")
	require.True(t, found)
	assert.Equal(t, "unknown_service:python", srcVal.Str())
	destVal, found := attrs.Get(metadataKeyLogGroup)
	require.True(t, found)
	assert.Equal(t, "/aws/service-events/unknown_service", destVal.Str())
}

func TestBuildOTTLSetStatementsUnknownServiceWithoutColon(t *testing.T) {
	statements := buildOTTLSetStatements(metadataKeyLogGroup, parseTemplate("/aws/service-events/{service.name}"))

	factory := transformprocessor.NewFactory()
	cfg := factory.CreateDefaultConfig().(*transformprocessor.Config)
	stmts := make([]interface{}, len(statements))
	for i, s := range statements {
		stmts[i] = s
	}
	cfgMap := map[string]interface{}{
		"log_statements": []interface{}{
			map[string]interface{}{
				"context":    "resource",
				"error_mode": "propagate",
				"statements": stmts,
			},
		},
	}
	require.NoError(t, confmap.NewFromStringMap(cfgMap).Unmarshal(&cfg))

	sink := new(consumertest.LogsSink)
	proc, err := factory.CreateLogs(gocontext.Background(), processortest.NewNopSettings(factory.Type()), cfg, sink)
	require.NoError(t, err)
	require.NoError(t, proc.Start(gocontext.Background(), nil))
	defer proc.Shutdown(gocontext.Background())

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "unknown_service")
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	err = proc.ConsumeLogs(gocontext.Background(), logs)
	require.NoError(t, err)

	result := sink.AllLogs()
	require.Equal(t, 1, len(result))
	attrs := result[0].ResourceLogs().At(0).Resource().Attributes()
	val, found := attrs.Get(metadataKeyLogGroup)
	require.True(t, found)
	assert.Equal(t, "/aws/service-events/unknown_service", val.Str())
}
