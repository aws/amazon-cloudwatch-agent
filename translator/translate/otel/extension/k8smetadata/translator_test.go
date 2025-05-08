// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8smetadata

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/extension/k8smetadata"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslate(t *testing.T) {
	testCases := map[string]struct {
		input map[string]interface{}
		name  string
		want  *k8smetadata.Config
	}{
		"DefaultConfig": {
			input: map[string]interface{}{},
			name:  "",
			want:  &k8smetadata.Config{},
		},
		"OTLPConfig": {
			input: map[string]interface{}{
				common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.OtlpKey): map[string]interface{}{},
			},
			name: "",
			want: &k8smetadata.Config{
				Objects: []string{"endpointslices"},
			},
		},
		"AppSignalsConfig": {
			input: map[string]interface{}{
				common.AppSignalsMetrics: map[string]interface{}{},
			},
			name: common.AppSignals,
			want: &k8smetadata.Config{
				Objects: []string{"endpointslices", "services"},
			},
		},
		"AppSignalsFallbackConfig": {
			input: map[string]interface{}{
				common.AppSignalsMetricsFallback: map[string]interface{}{},
			},
			name: common.AppSignalsFallback,
			want: &k8smetadata.Config{
				Objects: []string{"endpointslices", "services"},
			},
		},
		"BothOTLPAndAppSignals": {
			input: map[string]interface{}{
				common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.OtlpKey): map[string]interface{}{},
				common.AppSignalsMetrics: map[string]interface{}{},
			},
			name: common.AppSignals,
			want: &k8smetadata.Config{
				Objects: []string{"endpointslices", "services"},
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			tt := NewTranslator().(*translator)
			tt.name = testCase.name
			if tt.name == "" {
				assert.Equal(t, "k8smetadata", tt.ID().String())
			} else {
				assert.Equal(t, "k8smetadata/"+tt.name, tt.ID().String())
			}
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.NoError(t, err)
			assert.Equal(t, testCase.want, got)
		})
	}
}

func TestIsAppSignals(t *testing.T) {
	testCases := map[string]struct {
		input map[string]interface{}
		name  string
		want  bool
	}{
		"NotAppSignals": {
			input: map[string]interface{}{},
			name:  "",
			want:  false,
		},
		"AppSignalsMetrics": {
			input: map[string]interface{}{
				common.AppSignalsMetrics: map[string]interface{}{},
			},
			name: common.AppSignals,
			want: true,
		},
		"AppSignalsTraces": {
			input: map[string]interface{}{
				common.AppSignalsTraces: map[string]interface{}{},
			},
			name: common.AppSignals,
			want: true,
		},
		"AppSignalsFallbackMetrics": {
			input: map[string]interface{}{
				common.AppSignalsMetricsFallback: map[string]interface{}{},
			},
			name: common.AppSignalsFallback,
			want: true,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			tt := NewTranslator().(*translator)
			tt.name = testCase.name
			conf := confmap.NewFromStringMap(testCase.input)
			got := tt.isAppSignals(conf)
			assert.Equal(t, testCase.want, got)
		})
	}
}

func TestIsOTLP(t *testing.T) {
	testCases := map[string]struct {
		input map[string]interface{}
		want  bool
	}{
		"NotOTLP": {
			input: map[string]interface{}{},
			want:  false,
		},
		"OTLPLogs": {
			input: map[string]interface{}{
				common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.OtlpKey): map[string]interface{}{},
			},
			want: true,
		},
		"OTLPMetrics": {
			input: map[string]interface{}{
				common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.OtlpKey): map[string]interface{}{},
			},
			want: true,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			tt := NewTranslator().(*translator)
			conf := confmap.NewFromStringMap(testCase.input)
			got := tt.isOTLP(conf)
			assert.Equal(t, testCase.want, got)
		})
	}
}
