// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package host

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

type testTranslator struct {
	id component.ID
}

var _ common.ComponentTranslator = (*testTranslator)(nil)

func (t testTranslator) Translate(_ *confmap.Conf) (component.Config, error) {
	return nil, nil
}

func (t testTranslator) ID() component.ID {
	return t.id
}

func TestTranslator(t *testing.T) {
	type want struct {
		pipelineID string
		receivers  []string
		processors []string
		exporters  []string
		extensions []string
	}
	testCases := map[string]struct {
		input          map[string]interface{}
		pipelineName   string
		destination    string
		mode           string
		kubernetesMode string
		isECS          bool
		want           *want
		wantErr        error
	}{
		"WithMetricsSection": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{},
			},
			pipelineName: common.PipelineNameHost,
			mode:         config.ModeEC2,
			want: &want{
				pipelineID: "metrics/host",
				receivers:  []string{"nop", "other"},
				processors: []string{"awsentity/resource"},
				exporters:  []string{"awscloudwatch"},
				extensions: []string{"agenthealth/metrics", "agenthealth/statuscode"},
			},
		},
		"WithDeltaMetrics": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"net": map[string]interface{}{},
					},
				},
			},
			pipelineName: common.PipelineNameHostDeltaMetrics,
			mode:         config.ModeEC2,
			want: &want{
				pipelineID: "metrics/hostDeltaMetrics",
				receivers:  []string{"nop", "other"},
				processors: []string{"cumulativetodelta/hostDeltaMetrics", "awsentity/resource"},
				exporters:  []string{"awscloudwatch"},
				extensions: []string{"agenthealth/metrics", "agenthealth/statuscode"},
			},
		},
		"WithOtlpMetricsEC2": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"otlp": map[string]interface{}{},
					},
				},
			},
			pipelineName: common.PipelineNameHostOtlpMetrics,
			mode:         config.ModeEC2,
			want: &want{
				pipelineID: "metrics/hostOtlpMetrics",
				receivers:  []string{"nop", "other"},
				processors: []string{"cumulativetodelta/hostOtlpMetrics", "awsentity/service/otlp"},
				exporters:  []string{"awscloudwatch"},
				extensions: []string{"agenthealth/metrics", "agenthealth/statuscode"},
			},
		},
		"WithOtlpMetricsECS": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"otlp": map[string]interface{}{},
					},
				},
			},
			pipelineName: common.PipelineNameHostOtlpMetrics,
			mode:         config.ModeEC2,
			isECS:        true,
			want: &want{
				pipelineID: "metrics/hostOtlpMetrics",
				receivers:  []string{"nop", "other"},
				processors: []string{"cumulativetodelta/hostOtlpMetrics"},
				exporters:  []string{"awscloudwatch"},
				extensions: []string{"agenthealth/metrics", "agenthealth/statuscode"},
			},
		},
		"WithOtlpMetricsKubernetes": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"otlp": map[string]interface{}{},
					},
				},
			},
			pipelineName:   common.PipelineNameHostOtlpMetrics,
			kubernetesMode: config.ModeK8sEC2,
			want: &want{
				pipelineID: "metrics/hostOtlpMetrics",
				receivers:  []string{"nop", "other"},
				processors: []string{"cumulativetodelta/hostOtlpMetrics", "awsentity/service/otlp"},
				exporters:  []string{"awscloudwatch"},
				extensions: []string{"k8smetadata", "agenthealth/metrics", "agenthealth/statuscode"},
			},
		},
		"WithOtlpMetrics/CloudWatchLogsEC2": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"otlp": map[string]interface{}{},
					},
				},
			},
			pipelineName: common.PipelineNameHostOtlpMetrics,
			destination:  common.CloudWatchLogsKey,
			mode:         config.ModeEC2,
			want: &want{
				pipelineID: "metrics/hostOtlpMetrics/cloudwatchlogs",
				receivers:  []string{"nop", "other"},
				processors: []string{"cumulativetodelta/hostOtlpMetrics/cloudwatchlogs", "awsentity/service/otlp", "batch/hostOtlpMetrics/cloudwatchlogs"},
				exporters:  []string{"awsemf"},
				extensions: []string{"agenthealth/logs", "agenthealth/statuscode"},
			},
		},
		"WithOtlpMetrics/CloudWatchLogsECS": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"otlp": map[string]interface{}{},
					},
				},
			},
			pipelineName: common.PipelineNameHostOtlpMetrics,
			destination:  common.CloudWatchLogsKey,
			mode:         config.ModeEC2,
			isECS:        true,
			want: &want{
				pipelineID: "metrics/hostOtlpMetrics/cloudwatchlogs",
				receivers:  []string{"nop", "other"},
				processors: []string{"cumulativetodelta/hostOtlpMetrics/cloudwatchlogs", "batch/hostOtlpMetrics/cloudwatchlogs"},
				exporters:  []string{"awsemf"},
				extensions: []string{"agenthealth/logs", "agenthealth/statuscode"},
			},
		},
		"WithOtlpMetrics/CloudWatchLogsKubernetes": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"otlp": map[string]interface{}{},
					},
				},
			},
			pipelineName:   common.PipelineNameHostOtlpMetrics,
			destination:    common.CloudWatchLogsKey,
			kubernetesMode: config.ModeK8sEC2,
			want: &want{
				pipelineID: "metrics/hostOtlpMetrics/cloudwatchlogs",
				receivers:  []string{"nop", "other"},
				processors: []string{"cumulativetodelta/hostOtlpMetrics/cloudwatchlogs", "awsentity/service/otlp", "batch/hostOtlpMetrics/cloudwatchlogs"},
				exporters:  []string{"awsemf"},
				extensions: []string{"k8smetadata", "agenthealth/logs", "agenthealth/statuscode"},
			},
		},
		"WithMetricsKeyStatsD": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"statsd": map[string]interface{}{},
					},
				},
			},
			pipelineName: common.PipelineNameHostCustomMetrics,
			mode:         config.ModeEC2,
			want: &want{
				pipelineID: "metrics/hostCustomMetrics",
				receivers:  []string{"nop", "other"},
				processors: []string{"awsentity/service/telegraf"},
				exporters:  []string{"awscloudwatch"},
				extensions: []string{"agenthealth/metrics", "agenthealth/statuscode"},
			},
		},
		"WithMetricsKeyStatsDContainer": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"statsd": map[string]interface{}{},
					},
				},
			},
			pipelineName: common.PipelineNameHostCustomMetrics,
			mode:         config.ModeEC2,
			isECS:        true,
			want: &want{
				pipelineID: "metrics/hostCustomMetrics",
				receivers:  []string{"nop", "other"},
				processors: []string{},
				exporters:  []string{"awscloudwatch"},
				extensions: []string{"agenthealth/metrics", "agenthealth/statuscode"},
			},
		},
		"WithMetricDecoration": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"cpu": map[string]interface{}{
							"measurement": []interface{}{
								map[string]interface{}{
									"name":   "cpu_usage_idle",
									"rename": "CPU_USAGE_IDLE",
								},
							},
						},
					},
				},
			},
			pipelineName: common.PipelineNameHost,
			mode:         config.ModeEC2,
			want: &want{
				pipelineID: "metrics/host",
				receivers:  []string{"nop", "other"},
				processors: []string{"transform", "awsentity/resource"},
				exporters:  []string{"awscloudwatch"},
				extensions: []string{"agenthealth/metrics", "agenthealth/statuscode"},
			},
		},
		"WithoutMetricDecoration": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"cpu": map[string]interface{}{
							"measurement": []interface{}{
								"cpu_usage_guest",
							},
						},
					},
				},
			},
			pipelineName: common.PipelineNameHost,
			mode:         config.ModeEC2,
			want: &want{
				pipelineID: "metrics/host",
				receivers:  []string{"nop", "other"},
				processors: []string{"awsentity/resource"},
				exporters:  []string{"awscloudwatch"},
				extensions: []string{"agenthealth/metrics", "agenthealth/statuscode"},
			},
		},
		"WithAppendDimensions": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"append_dimensions": map[string]interface{}{},
				},
			},
			pipelineName: common.PipelineNameHost,
			mode:         config.ModeEC2,
			want: &want{
				pipelineID: "metrics/host",
				receivers:  []string{"nop", "other"},
				processors: []string{"ec2tagger", "awsentity/resource"},
				exporters:  []string{"awscloudwatch"},
				extensions: []string{"agenthealth/metrics", "agenthealth/statuscode"},
			},
		},
		"WithPRWExporter/Aggregation": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"aggregation_dimensions": []interface{}{[]interface{}{"d1", "d2"}},
				},
			},
			pipelineName: common.PipelineNameHost,
			destination:  common.AMPKey,
			mode:         config.ModeEC2,
			want: &want{
				pipelineID: "metrics/host/amp",
				receivers:  []string{"nop", "other"},
				processors: []string{"rollup", "batch/host/amp", "deltatocumulative/host/amp"},
				exporters:  []string{"prometheusremotewrite/amp"},
				extensions: []string{"sigv4auth"},
			},
		},
		"WithPRWExporter/NoAggregation": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{},
			},
			pipelineName: common.PipelineNameHost,
			destination:  common.AMPKey,
			mode:         config.ModeEC2,
			want: &want{
				pipelineID: "metrics/host/amp",
				receivers:  []string{"nop", "other"},
				processors: []string{"batch/host/amp", "deltatocumulative/host/amp"},
				exporters:  []string{"prometheusremotewrite/amp"},
				extensions: []string{"sigv4auth"},
			},
		},
		"WithOtlpMetricsEC2AndServiceName": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"otlp": map[string]interface{}{
							"service.name": "test-service",
						},
					},
				},
			},
			pipelineName: common.PipelineNameHostOtlpMetrics,
			mode:         config.ModeEC2,
			want: &want{
				pipelineID: "metrics/hostOtlpMetrics",
				receivers:  []string{"nop", "other"},
				processors: []string{"cumulativetodelta/hostOtlpMetrics", "awsentity/service/otlp/cloudwatch"},
				exporters:  []string{"awscloudwatch"},
				extensions: []string{"agenthealth/metrics", "agenthealth/statuscode"},
			},
		},
		"WithOtlpMetricsEC2CloudWatchLogsAndServiceName": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"otlp": map[string]interface{}{
							"service.name": "test-service",
						},
					},
				},
			},
			pipelineName: common.PipelineNameHostOtlpMetrics,
			destination:  common.CloudWatchLogsKey,
			mode:         config.ModeEC2,
			want: &want{
				pipelineID: "metrics/hostOtlpMetrics/cloudwatchlogs",
				receivers:  []string{"nop", "other"},
				processors: []string{"cumulativetodelta/hostOtlpMetrics/cloudwatchlogs", "awsentity/service/otlp/cloudwatchlogs", "batch/hostOtlpMetrics/cloudwatchlogs"},
				exporters:  []string{"awsemf"},
				extensions: []string{"agenthealth/logs", "agenthealth/statuscode"},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			resetContext()
			if testCase.mode != "" {
				context.CurrentContext().SetMode(testCase.mode)
			}
			if testCase.kubernetesMode != "" {
				context.CurrentContext().SetKubernetesMode(testCase.kubernetesMode)
				context.CurrentContext().SetRunInContainer(true)
			}
			if testCase.isECS {
				ecsutil.GetECSUtilSingleton().Region = "test-region"
				context.CurrentContext().SetRunInContainer(true)
			}
			ht := NewTranslator(
				testCase.pipelineName,
				common.NewTranslatorMap[component.Config](
					&testTranslator{id: component.NewID(component.MustNewType("nop"))},
					&testTranslator{id: component.NewID(component.MustNewType("other"))},
				),
				common.WithDestination(testCase.destination),
			)
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := ht.Translate(conf)
			require.Equal(t, testCase.wantErr, err)
			if testCase.want == nil {
				require.Nil(t, got)
			} else {
				require.NotNil(t, got)
				require.EqualValues(t, testCase.want.pipelineID, ht.ID().String())
				assert.Equal(t, testCase.want.receivers, collections.MapSlice(got.Receivers.Keys(), component.ID.String))
				assert.Equal(t, testCase.want.processors, collections.MapSlice(got.Processors.Keys(), component.ID.String))
				assert.Equal(t, testCase.want.exporters, collections.MapSlice(got.Exporters.Keys(), component.ID.String))
				assert.Equal(t, testCase.want.extensions, collections.MapSlice(got.Extensions.Keys(), component.ID.String))
			}
		})
	}
}

func resetContext() {
	context.ResetContext()
	ecsutil.GetECSUtilSingleton().Region = ""
	context.CurrentContext().SetRunInContainer(false)
}
