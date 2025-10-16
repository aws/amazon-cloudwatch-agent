// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package applicationsignals

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
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
	tt := NewTranslator(pipeline.SignalTraces)
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
	tt := NewTranslator(pipeline.SignalMetrics)
	assert.EqualValues(t, "metrics/application_signals", tt.ID().String())
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
				receivers:  []string{"otlp/grpc_0_0_0_0_4315", "otlp/http_0_0_0_0_4316"},
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
				receivers:  []string{"otlp/grpc_0_0_0_0_4315", "otlp/http_0_0_0_0_4316"},
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
				receivers:  []string{"otlp/grpc_0_0_0_0_4315", "otlp/http_0_0_0_0_4316"},
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
	tt := NewTranslator(pipeline.SignalMetrics)
	assert.EqualValues(t, "metrics/application_signals", tt.ID().String())
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
				receivers:  []string{"otlp/grpc_0_0_0_0_4315", "otlp/http_0_0_0_0_4316"},
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
				receivers:  []string{"otlp/grpc_0_0_0_0_4315", "otlp/http_0_0_0_0_4316"},
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
	tt := NewTranslator(pipeline.SignalMetrics)
	assert.EqualValues(t, "metrics/application_signals", tt.ID().String())
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
				receivers:  []string{"otlp/grpc_0_0_0_0_4315", "otlp/http_0_0_0_0_4316"},
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
				receivers:  []string{"otlp/grpc_0_0_0_0_4315", "otlp/http_0_0_0_0_4316"},
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
