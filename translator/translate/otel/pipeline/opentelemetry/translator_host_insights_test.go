// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package opentelemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestHostInsightsTranslator(t *testing.T) {
	tt := NewHostInsightsTranslator()
	assert.EqualValues(t, "metrics/host_insights", tt.ID().String())

	testCases := map[string]struct {
		input   map[string]interface{}
		wantErr error
	}{
		"WithNilConf": {
			input:   nil,
			wantErr: &common.MissingKeyError{ID: tt.ID(), JsonKey: hostInsightsKey},
		},
		"WithoutHostInsightsKey": {
			input:   map[string]interface{}{},
			wantErr: &common.MissingKeyError{ID: tt.ID(), JsonKey: hostInsightsKey},
		},
		"WithHostInsightsKey": {
			input: map[string]interface{}{
				"opentelemetry": map[string]interface{}{
					"collect": map[string]interface{}{
						"host_insights": map[string]interface{}{},
					},
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var conf *confmap.Conf
			if tc.input != nil {
				conf = confmap.NewFromStringMap(tc.input)
			}
			got, err := tt.Translate(conf)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.Equal(t, tc.wantErr, err)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, 1, got.Receivers.Len())
				assert.Equal(t, 0, got.Processors.Len())
				assert.Equal(t, 1, got.Exporters.Len())
				assert.Equal(t, 0, got.Extensions.Len())
				assert.Equal(t, 1, got.Connectors.Len())
				assert.Equal(t, "hostmetrics", got.Receivers.Keys()[0].String())
				assert.Equal(t, "forward/otel", got.Exporters.Keys()[0].String())
				assert.Equal(t, "forward/otel", got.Connectors.Keys()[0].String())
			}
		})
	}
}
