// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package xray

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

func TestTranslator(t *testing.T) {
	type want struct {
		receivers  []string
		processors []string
		exporters  []string
		extensions []string
	}
	tt := NewTranslator()
	assert.EqualValues(t, "traces/xray", tt.ID().String())
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *want
		wantErr error
	}{
		"WithoutTracesCollectedKey": {
			input:   map[string]interface{}{},
			wantErr: &common.MissingKeyError{ID: tt.ID(), JsonKey: fmt.Sprint(xrayKey, " or ", otlpKey)},
		},
		"WithXrayKey": {
			input: map[string]interface{}{
				"traces": map[string]interface{}{
					"traces_collected": map[string]interface{}{
						"xray": nil,
					},
				},
			},
			want: &want{
				receivers:  []string{"awsxray"},
				processors: []string{"batch/xray"},
				exporters:  []string{"awsxray"},
				extensions: []string{"agenthealth/traces"},
			},
		},
		"WithOtlpKey": {
			input: map[string]interface{}{
				"traces": map[string]interface{}{
					"traces_collected": map[string]interface{}{
						"otlp": nil,
					},
				},
			},
			want: &want{
				receivers:  []string{"otlp/traces"},
				processors: []string{"batch/xray"},
				exporters:  []string{"awsxray"},
				extensions: []string{"agenthealth/traces"},
			},
		},
		"WithXrayAndOtlpKey": {
			input: map[string]interface{}{
				"traces": map[string]interface{}{
					"traces_collected": map[string]interface{}{
						"xray": nil,
						"otlp": nil,
					},
				},
			},
			want: &want{
				receivers:  []string{"awsxray", "otlp/traces"},
				processors: []string{"batch/xray"},
				exporters:  []string{"awsxray"},
				extensions: []string{"agenthealth/traces"},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if testCase.want == nil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, testCase.want.receivers, collections.MapSlice(got.Receivers.Keys(), component.ID.String))
				assert.Equal(t, testCase.want.processors, collections.MapSlice(got.Processors.Keys(), component.ID.String))
				assert.Equal(t, testCase.want.exporters, collections.MapSlice(got.Exporters.Keys(), component.ID.String))
				assert.Equal(t, testCase.want.extensions, collections.MapSlice(got.Extensions.Keys(), component.ID.String))
			}
		})
	}
}
