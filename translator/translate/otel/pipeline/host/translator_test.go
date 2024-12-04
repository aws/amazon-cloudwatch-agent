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
)

type testTranslator struct {
	id component.ID
}

var _ common.Translator[component.Config] = (*testTranslator)(nil)

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
		runInContainer bool
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
				extensions: []string{"agenthealth/metrics"},
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
				processors: []string{"awsentity/resource", "cumulativetodelta/hostDeltaMetrics"},
				exporters:  []string{"awscloudwatch"},
				extensions: []string{"agenthealth/metrics"},
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
				extensions: []string{"agenthealth/metrics"},
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
			pipelineName:   common.PipelineNameHostCustomMetrics,
			mode:           config.ModeEC2,
			runInContainer: true,
			want: &want{
				pipelineID: "metrics/hostCustomMetrics",
				receivers:  []string{"nop", "other"},
				processors: []string{},
				exporters:  []string{"awscloudwatch"},
				extensions: []string{"agenthealth/metrics"},
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
				processors: []string{"awsentity/resource", "transform"},
				exporters:  []string{"awscloudwatch"},
				extensions: []string{"agenthealth/metrics"},
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
				extensions: []string{"agenthealth/metrics"},
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
				processors: []string{"awsentity/resource", "ec2tagger"},
				exporters:  []string{"awscloudwatch"},
				extensions: []string{"agenthealth/metrics"},
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
				processors: []string{"rollup", "batch/host/amp"},
				exporters:  []string{"prometheusremotewrite/amp"},
				extensions: []string{"sigv4auth", "agenthealth/metrics"},
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
				processors: []string{"batch/host/amp"},
				exporters:  []string{"prometheusremotewrite/amp"},
				extensions: []string{"sigv4auth", "agenthealth/metrics"},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			context.CurrentContext().SetMode(testCase.mode)
			context.CurrentContext().SetRunInContainer(testCase.runInContainer)
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
