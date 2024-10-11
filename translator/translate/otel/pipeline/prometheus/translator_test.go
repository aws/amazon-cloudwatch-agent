// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	type want struct {
		pipelineID string
		receivers  []string
		processors []string
		exporters  []string
		extensions []string
	}
	testCases := map[string]struct {
		input       map[string]any
		index       int
		destination string
		dataType    component.DataType
		want        *want
		wantErr     error
	}{
		"WithoutPrometheusMetrics": {
			input:    map[string]any{},
			dataType: component.DataTypeMetrics,
			wantErr: &common.MissingKeyError{
				ID:      component.NewIDWithName(component.DataTypeMetrics, "prometheus"),
				JsonKey: "metrics::metrics_collected::prometheus",
			},
		},
		"WithoutPrometheusLogs": {
			input:    map[string]any{},
			dataType: component.DataTypeLogs,
			wantErr: &common.MissingKeyError{
				ID:      component.NewIDWithName(component.DataTypeLogs, "prometheus"),
				JsonKey: "logs::metrics_collected::prometheus",
			},
		},
		"WithEmptyPath": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_collected": map[string]any{
						"prometheus": map[string]any{},
					},
				},
			},
			dataType: component.DataTypeMetrics,
			wantErr: &common.MissingKeyError{
				ID:      component.NewIDWithName(component.DataTypeMetrics, "prometheus"),
				JsonKey: "metrics::metrics_collected::prometheus::prometheus_config_path",
			},
		},
		"WithMissingDestinations": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_collected": map[string]any{
						"prometheus": map[string]any{
							"prometheus_config_path": "test.yaml",
						},
					},
				},
			},
			dataType:    component.DataTypeMetrics,
			destination: common.AMPKey,
			wantErr:     errors.New("pipeline (prometheus/amp) does not have destination (amp) in configuration"),
		},
		"WithValidAMP": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_destinations": map[string]any{
						"amp": map[string]any{
							"workspace_id": "ws1234",
						},
					},
					"metrics_collected": map[string]any{
						"prometheus": map[string]any{
							"prometheus_config_path": "test.yaml",
						},
					},
				},
			},
			dataType:    component.DataTypeMetrics,
			destination: common.AMPKey,
			want: &want{
				pipelineID: "metrics/prometheus/amp",
				receivers:  []string{"prometheus"},
				processors: []string{"batch/prometheus/amp"},
				exporters:  []string{"prometheusremotewrite/amp"},
				extensions: []string{"sigv4auth"},
			},
		},
		"WithValidCloudWatch": {
			input: map[string]any{
				"logs": map[string]any{
					"metrics_collected": map[string]any{
						"prometheus": map[string]any{},
					},
				},
			},
			dataType:    component.DataTypeLogs,
			destination: common.CloudWatchKey,
			want: &want{
				pipelineID: "logs/prometheus/cloudwatch",
				receivers:  []string{"telegraf_prometheus"},
				processors: []string{"batch/prometheus/cloudwatch"},
				exporters:  []string{"awsemf/prometheus"},
				extensions: []string{"agenthealth/logs"},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			tt := NewTranslator(WithDataType(testCase.dataType), WithDestination(testCase.destination))
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			require.Equal(t, testCase.wantErr, err)
			if testCase.want == nil {
				require.Nil(t, got)
			} else {
				require.NotNil(t, got)
				require.EqualValues(t, testCase.want.pipelineID, tt.ID().String())
				assert.Equal(t, testCase.want.receivers, collections.MapSlice(got.Receivers.Keys(), component.ID.String))
				assert.Equal(t, testCase.want.processors, collections.MapSlice(got.Processors.Keys(), component.ID.String))
				assert.Equal(t, testCase.want.exporters, collections.MapSlice(got.Exporters.Keys(), component.ID.String))
				assert.Equal(t, testCase.want.extensions, collections.MapSlice(got.Extensions.Keys(), component.ID.String))
			}
		})
	}
}
