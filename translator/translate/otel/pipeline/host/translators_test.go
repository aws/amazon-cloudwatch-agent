// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package host

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	translatorcontext "github.com/aws/amazon-cloudwatch-agent/translator"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/registerrules"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslators(t *testing.T) {
	type want struct {
		receivers []string
		exporters []string
	}
	testCases := map[string]struct {
		input         map[string]any
		configSection string
		want          map[string]want
	}{
		"WithEmpty": {
			input:         map[string]any{},
			configSection: MetricsKey,
			want:          map[string]want{},
		},
		"WithMinimal": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_collected": map[string]any{
						"cpu": map[string]any{},
					},
				},
			},
			configSection: MetricsKey,
			want: map[string]want{
				"metrics/host": {
					receivers: []string{"telegraf_cpu"},
					exporters: []string{"awscloudwatch"},
				},
			},
		},
		"WithAMPDestination": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_destinations": map[string]any{
						"amp": map[string]any{
							"workspace_id": "ws-12345",
						},
					},
					"metrics_collected": map[string]any{
						"cpu": map[string]any{},
					},
				},
			},
			configSection: MetricsKey,
			want: map[string]want{
				"metrics/host/amp": {
					receivers: []string{"telegraf_cpu"},
					exporters: []string{"prometheusremotewrite/amp"},
				},
			},
		},
		"WithAMPAndCloudWatchDestinations": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_destinations": map[string]any{
						"amp": map[string]any{
							"workspace_id": "ws-12345",
						},
						"cloudwatch": map[string]any{},
					},
					"metrics_collected": map[string]any{
						"cpu": map[string]any{},
					},
				},
			},
			configSection: MetricsKey,
			want: map[string]want{
				"metrics/host/cloudwatch": {
					receivers: []string{"telegraf_cpu"},
					exporters: []string{"awscloudwatch"},
				},
				"metrics/host/amp": {
					receivers: []string{"telegraf_cpu"},
					exporters: []string{"prometheusremotewrite/amp"},
				},
			},
		},
		"WithDeltaMetrics": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_destinations": map[string]any{
						"amp": map[string]any{
							"workspace_id": "ws-12345",
						},
						"cloudwatch": map[string]any{},
					},
					"metrics_collected": map[string]any{
						"net": map[string]any{},
					},
				},
			},
			configSection: MetricsKey,
			want: map[string]want{
				"metrics/hostDeltaMetrics/cloudwatch": {
					receivers: []string{"telegraf_net"},
					exporters: []string{"awscloudwatch"},
				},
				"metrics/host/amp": {
					receivers: []string{"telegraf_net"},
					exporters: []string{"prometheusremotewrite/amp"},
				},
			},
		},
		"WithBothEbsAndAdaptedDiskio": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_destinations": map[string]any{
						"amp": map[string]any{
							"workspace_id": "ws-12345",
						},
						"cloudwatch": map[string]any{},
					},
					"metrics_collected": map[string]any{
						"diskio": map[string]any{
							"measurement": []interface{}{"io_time", "ebs_total_read_bytes"},
						},
					},
				},
			},
			configSection: MetricsKey,
			want: map[string]want{
				"metrics/hostDeltaMetrics/cloudwatch": {
					receivers: []string{"telegraf_diskio", "awsnvmereceiver"},
					exporters: []string{"awscloudwatch"},
				},
				"metrics/host/amp": {
					receivers: []string{"telegraf_diskio", "awsnvmereceiver"},
					exporters: []string{"prometheusremotewrite/amp"},
				},
			},
		},
		"WithoutEbsoMetrics": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_destinations": map[string]any{
						"amp": map[string]any{
							"workspace_id": "ws-12345",
						},
						"cloudwatch": map[string]any{},
					},
					"metrics_collected": map[string]any{
						"diskio": map[string]any{
							"measurement": []interface{}{"io_time"},
						},
					},
				},
			},
			configSection: MetricsKey,
			want: map[string]want{
				"metrics/hostDeltaMetrics/cloudwatch": {
					receivers: []string{"telegraf_diskio"},
					exporters: []string{"awscloudwatch"},
				},
				"metrics/host/amp": {
					receivers: []string{"telegraf_diskio"},
					exporters: []string{"prometheusremotewrite/amp"},
				},
			},
		},
		"WithEbsMetrics": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_destinations": map[string]any{
						"amp": map[string]any{
							"workspace_id": "ws-12345",
						},
						"cloudwatch": map[string]any{},
					},
					"metrics_collected": map[string]any{
						"diskio": map[string]any{
							"measurement": []interface{}{"ebs_total_read_bytes"},
						},
					},
				},
			},
			configSection: MetricsKey,
			want: map[string]want{
				"metrics/hostDeltaMetrics/cloudwatch": {
					receivers: []string{"awsnvmereceiver"},
					exporters: []string{"awscloudwatch"},
				},
				"metrics/host/amp": {
					receivers: []string{"awsnvmereceiver"},
					exporters: []string{"prometheusremotewrite/amp"},
				},
			},
		},
		"WithOtlpMetrics/CloudWatch": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_collected": map[string]any{
						"otlp": map[string]any{},
					},
				},
			},
			configSection: MetricsKey,
			want: map[string]want{
				"metrics/hostOtlpMetrics": {
					receivers: []string{"otlp/metrics"},
					exporters: []string{"awscloudwatch"},
				},
			},
		},
		"WithOtlpMetrics/CloudWatchLogs": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"otlp": map[string]interface{}{},
					},
				},
			},
			configSection: LogsKey,
			want: map[string]want{
				"metrics/hostOtlpMetrics/cloudwatchlogs": {
					receivers: []string{"otlp/metrics"},
					exporters: []string{"awsemf"},
				},
			},
		},
		"WithCustomMetrics": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"statsd": map[string]interface{}{},
					},
				},
			},
			configSection: MetricsKey,
			want: map[string]want{
				"metrics/hostCustomMetrics": {
					receivers: []string{"telegraf_statsd"},
					exporters: []string{"awscloudwatch"},
				},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			translatorcontext.SetTargetPlatform("linux")
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := NewTranslators(conf, testCase.configSection, "linux")
			require.NoError(t, err)
			if testCase.want == nil {
				require.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, len(testCase.want), got.Len())
				got.Range(func(tr common.Translator[*common.ComponentTranslators, pipeline.ID]) {
					w, ok := testCase.want[tr.ID().String()]
					require.True(t, ok)
					g, err := tr.Translate(conf)
					assert.NoError(t, err)
					assert.Equal(t, w.receivers, collections.MapSlice(g.Receivers.Keys(), component.ID.String))
					assert.Equal(t, w.exporters, collections.MapSlice(g.Exporters.Keys(), component.ID.String))
				})
			}
		})
	}
}

func TestTranslatorsError(t *testing.T) {
	got, err := NewTranslators(confmap.New(), MetricsKey, "invalid")
	assert.Error(t, err)
	assert.Nil(t, got)
}
