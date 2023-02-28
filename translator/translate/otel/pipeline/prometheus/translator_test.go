// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/internal/util/collections"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	type want struct {
		receivers  []string
		processors []string
		exporters  []string
	}
	cit := NewTranslator()
	require.EqualValues(t, "metrics/prometheus", cit.ID().String())
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *want
		wantErr error
	}{
		"WithoutPrometheusKey": {
			input:   map[string]interface{}{},
			wantErr: &common.MissingKeyError{ID: cit.ID(), JsonKey: "logs::metrics_collected::prometheus"},
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
				receivers:  []string{"prometheus/prometheus"},
				processors: []string{"batch/prometheus", "metricstransform/prometheus", "resource/prometheus"},
				exporters:  []string{"awsemf/prometheus"},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := cit.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if testCase.want == nil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, testCase.want.receivers, collections.MapSlice(got.Receivers.SortedKeys(), toString))
				assert.Equal(t, testCase.want.processors, collections.MapSlice(got.Processors.SortedKeys(), toString))
				assert.Equal(t, testCase.want.exporters, collections.MapSlice(got.Exporters.SortedKeys(), toString))
			}
		})
	}
}

func toString(id component.ID) string {
	return id.String()
}
