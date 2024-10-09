// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package host

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
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
		input        map[string]interface{}
		pipelineName string
		destination  string
		want         *want
		wantErr      error
	}{
		"WithMetricsSection": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{},
			},
			pipelineName: common.PipelineNameHost,
			want: &want{
				pipelineID: "metrics/host",
				receivers:  []string{"nop", "other"},
				processors: []string{},
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
			pipelineName: fmt.Sprintf("%s_test", common.PipelineNameHostDeltaMetrics),
			want: &want{
				pipelineID: "metrics/hostDeltaMetrics_test",
				receivers:  []string{"nop", "other"},
				processors: []string{"cumulativetodelta/hostDeltaMetrics_test"},
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
			want: &want{
				pipelineID: "metrics/host",
				receivers:  []string{"nop", "other"},
				processors: []string{"transform"},
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
			want: &want{
				pipelineID: "metrics/host",
				receivers:  []string{"nop", "other"},
				processors: []string{},
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
			want: &want{
				pipelineID: "metrics/host",
				receivers:  []string{"nop", "other"},
				processors: []string{"ec2tagger"},
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
			want: &want{
				pipelineID: "metrics/host/amp",
				receivers:  []string{"nop", "other"},
				processors: []string{"rollup", "batch/host/amp"},
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
			want: &want{
				pipelineID: "metrics/host/amp",
				receivers:  []string{"nop", "other"},
				processors: []string{"batch/host/amp"},
				exporters:  []string{"prometheusremotewrite/amp"},
				extensions: []string{"sigv4auth"},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
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
