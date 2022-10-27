// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package host

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
	ht := NewTranslator([]config.Type{"other", "nop"})
	require.EqualValues(t, "host", ht.Type())
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *want
		wantErr error
	}{
		"WithoutMetricsKey": {
			input:   map[string]interface{}{},
			wantErr: &common.MissingKeyError{Type: "host", JsonKey: "metrics"},
		},
		"WithMetricsKey": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{},
			},
			want: &want{
				pipelineType: "metrics/host",
				receivers:    []string{"nop", "other"},
				processors:   []string{"cumulativetodelta/host"},
				exporters:    []string{"awscloudwatch/host"},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
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

func toString(id config.ComponentID) string {
	return id.String()
}
