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

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/eksdetector"
)

func TestTranslatorTraces(t *testing.T) {
	type want struct {
		receivers  []string
		processors []string
		exporters  []string
		extensions []string
	}
	tt := NewTranslator(component.DataTypeTraces)
	assert.EqualValues(t, "traces/application_signals", tt.ID().String())
	testCases := map[string]struct {
		input      map[string]interface{}
		want       *want
		wantErr    error
		detector   func() (eksdetector.Detector, error)
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
				receivers:  []string{"otlp/application_signals"},
				processors: []string{"resourcedetection", "awsapplicationsignals"},
				exporters:  []string{"awsxray/application_signals"},
				extensions: []string{"awsproxy/application_signals", "agenthealth/traces"},
			},
			detector:   eksdetector.TestEKSDetector,
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
				receivers:  []string{"otlp/application_signals"},
				processors: []string{"resourcedetection", "awsapplicationsignals"},
				exporters:  []string{"awsxray/application_signals"},
				extensions: []string{"awsproxy/application_signals", "agenthealth/traces"},
			},
			detector:   eksdetector.TestK8sDetector,
			isEKSCache: eksdetector.TestIsEKSCacheK8s,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Setenv(common.KubernetesEnvVar, "TEST")
			eksdetector.NewDetector = testCase.detector
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
	tt := NewTranslator(component.DataTypeMetrics)
	assert.EqualValues(t, "metrics/application_signals", tt.ID().String())
	testCases := map[string]struct {
		input      map[string]interface{}
		want       *want
		wantErr    error
		detector   func() (eksdetector.Detector, error)
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
				receivers:  []string{"otlp/application_signals"},
				processors: []string{"resourcedetection", "awsapplicationsignals"},
				exporters:  []string{"awsemf/application_signals"},
				extensions: []string{"agenthealth/logs"},
			},
			detector:   eksdetector.TestEKSDetector,
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
				receivers:  []string{"otlp/application_signals"},
				processors: []string{"resourcedetection", "awsapplicationsignals"},
				exporters:  []string{"debug/application_signals", "awsemf/application_signals"},
				extensions: []string{"agenthealth/logs"},
			},
			detector:   eksdetector.TestEKSDetector,
			isEKSCache: eksdetector.TestIsEKSCacheEKS,
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
				receivers:  []string{"otlp/application_signals"},
				processors: []string{"resourcedetection", "awsapplicationsignals"},
				exporters:  []string{"awsemf/application_signals"},
				extensions: []string{"agenthealth/logs"},
			},
			detector:   eksdetector.TestK8sDetector,
			isEKSCache: eksdetector.TestIsEKSCacheK8s,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Setenv(common.KubernetesEnvVar, "TEST")
			eksdetector.NewDetector = testCase.detector
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
func TestTranslatorMetricsForEC2(t *testing.T) {
	type want struct {
		receivers  []string
		processors []string
		exporters  []string
		extensions []string
	}
	tt := NewTranslator(component.DataTypeMetrics)
	assert.EqualValues(t, "metrics/application_signals", tt.ID().String())
	testCases := map[string]struct {
		input      map[string]interface{}
		want       *want
		wantErr    error
		detector   func() (eksdetector.Detector, error)
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
				receivers:  []string{"otlp/application_signals"},
				processors: []string{"resourcedetection", "awsapplicationsignals"},
				exporters:  []string{"awsemf/application_signals"},
				extensions: []string{"agenthealth/logs"},
			},
			detector:   eksdetector.TestEKSDetector,
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
				receivers:  []string{"otlp/application_signals"},
				processors: []string{"resourcedetection", "awsapplicationsignals"},
				exporters:  []string{"debug/application_signals", "awsemf/application_signals"},
				extensions: []string{"agenthealth/logs"},
			},
			detector:   eksdetector.TestEKSDetector,
			isEKSCache: eksdetector.TestIsEKSCacheEKS,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx := context.CurrentContext()
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
