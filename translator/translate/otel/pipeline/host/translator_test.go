// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package host

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/internal/util/collections"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	type want struct {
		pipelineType string
		receivers    []string
		processors   []string
		exporters    []string
	}
	testCases := map[string]struct {
		input        map[string]interface{}
		pipelineName component.Type
		want         *want
		wantErr      error
	}{
		"WithoutMetricsKey": {
			input:        map[string]interface{}{},
			pipelineName: common.HostPipelineName,
			wantErr:      &common.MissingKeyError{Type: common.HostPipelineName, JsonKey: common.MetricsKey},
		},
		"WithMetricsKey": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{},
			},
			pipelineName: common.HostPipelineName,
			want: &want{
				pipelineType: "metrics/host",
				receivers:    []string{"nop", "other"},
				processors:   []string{},
				exporters:    []string{"awscloudwatch"},
			},
		},
		"WithMetricsKeyNet": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"net": map[string]interface{}{},
					},
				},
			},
			pipelineName: common.HostDeltaMetricsPipelineName,
			want: &want{
				pipelineType: "metrics/hostDeltaMetrics",
				receivers:    []string{"nop", "other"},
				processors:   []string{"cumulativetodelta/hostDeltaMetrics"},
				exporters:    []string{"awscloudwatch"},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			ht := NewTranslator([]component.Type{"other", "nop"}, testCase.pipelineName)
			require.EqualValues(t, testCase.pipelineName, ht.Type())
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := ht.Translate(conf)
			require.Equal(t, testCase.wantErr, err)
			if testCase.want == nil {
				require.Nil(t, got)
			} else {
				require.EqualValues(t, testCase.want.pipelineType, got.Key.String())
				require.Equal(t, testCase.want.receivers, collections.MapSlice(got.Value.Receivers, toString))
				require.Equal(t, testCase.want.processors, collections.MapSlice(got.Value.Processors, toString))
				require.Equal(t, testCase.want.exporters, collections.MapSlice(got.Value.Exporters, toString))
			}
		})
	}
}

func toString(id component.ID) string {
	return id.String()
}
