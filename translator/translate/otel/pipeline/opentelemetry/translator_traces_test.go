// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package opentelemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestBaseTracesTranslator(t *testing.T) {
	tt := NewBaseTracesTranslator()
	assert.EqualValues(t, "traces/opentelemetry", tt.ID().String())

	otlpTracesKey := common.ConfigKey(common.OpenTelemetryKey, common.CollectKey, common.OtlpKey)
	testCases := map[string]struct {
		input   map[string]interface{}
		wantErr error
	}{
		"WithNilConf": {
			input:   nil,
			wantErr: &common.MissingKeyError{ID: tt.ID(), JsonKey: otlpTracesKey},
		},
		"WithoutOtlpKey": {
			input:   map[string]interface{}{},
			wantErr: &common.MissingKeyError{ID: tt.ID(), JsonKey: otlpTracesKey},
		},
		"WithOtlpKey": {
			input: map[string]interface{}{
				"opentelemetry": map[string]interface{}{
					"collect": map[string]interface{}{
						"otlp": map[string]interface{}{},
					},
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			agent.Global_Config.Region = "us-west-2"
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
				assert.Equal(t, 2, got.Processors.Len())
				assert.Equal(t, 1, got.Exporters.Len())
				assert.Equal(t, 1, got.Extensions.Len())
				assert.Equal(t, 1, got.Connectors.Len())
				assert.Equal(t, "forward/opentelemetry", got.Receivers.Keys()[0].String())
				assert.Equal(t, "otlphttp/traces", got.Exporters.Keys()[0].String())
				assert.Equal(t, "sigv4auth/xray", got.Extensions.Keys()[0].String())
				assert.Equal(t, "forward/opentelemetry", got.Connectors.Keys()[0].String())
			}
		})
	}
}

func TestBaseTracesTranslatorEmptyRegion(t *testing.T) {
	agent.Global_Config.Region = ""
	tt := NewBaseTracesTranslator()
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"otlp": map[string]interface{}{},
			},
		},
	})
	got, err := tt.Translate(conf)
	require.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "region is required")
}
