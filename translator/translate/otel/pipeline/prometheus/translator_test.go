// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
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
	cit := NewTranslator()
	require.EqualValues(t, "prometheus", cit.Type())
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *want
		wantErr error
	}{
		"WithoutPrometheusKey": {
			input:   map[string]interface{}{},
			wantErr: &common.MissingKeyError{Type: "prometheus", JsonKey: "logs::metrics_collected::prometheus"},
		},
		"WithPrometheusKey": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"prometheus": nil,
					},
				},
			},
			want: &want{
				pipelineType: "metrics/prometheus",
				receivers:    []string{"prometheus/prometheus"},
				processors:   []string{"batch/prometheus", "resource/prometheus", "metricstransform/prometheus"},
				exporters:    []string{"awsemf/prometheus"},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := cit.Translate(conf)
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

func toString(id config.ComponentID) string {
	return id.String()
}
