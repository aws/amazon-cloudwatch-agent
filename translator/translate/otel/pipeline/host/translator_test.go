// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package host

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap"
)

func TestNewTranslator(t *testing.T) {
	type want struct {
		pipelineType string
		receivers    []string
		processors   []string
		exporters    []string
	}
	ht := NewTranslator()
	require.EqualValues(t, "host", ht.Type())
	testCases := map[string]struct {
		input map[string]interface{}
		want  *want
	}{
		"WithoutKey": {
			input: map[string]interface{}{},
		},
		"WithMetricsKey": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{},
			},
			want: &want{
				pipelineType: "metrics/host",
				receivers:    []string{"telegraf_cpu"},
				processors:   []string{"cumulativetodelta/host"},
				exporters:    []string{"awscloudwatch/host"},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, _ := ht.Translate(conf)
			if testCase.want == nil {
				require.Nil(t, got)
			} else {
				require.EqualValues(t, testCase.want.pipelineType, got.Key.String())
				require.Equal(t, testCase.want.receivers, toStringSlice(got.Value.Receivers))
				require.Equal(t, testCase.want.processors, toStringSlice(got.Value.Processors))
				require.Equal(t, testCase.want.exporters, toStringSlice(got.Value.Exporters))
			}
		})
	}
}

func toStringSlice(ids []config.ComponentID) []string {
	var values []string
	for _, id := range ids {
		values = append(values, id.String())
	}
	return values
}
