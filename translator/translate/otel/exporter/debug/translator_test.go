// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package debug

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter/debugexporter"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	tt := NewTranslator()
	assert.EqualValues(t, "debug/application_signals", tt.ID().String())
	got, err := tt.Translate(confmap.New())
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestTranslate(t *testing.T) {
	tt := NewTranslator()
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *confmap.Conf
		wantErr error
	}{
		"WithMissingKey": {
			input: map[string]interface{}{"logs": map[string]interface{}{}},
			wantErr: &common.MissingKeyError{
				ID:      tt.ID(),
				JsonKey: common.AgentDebugConfigKey,
			},
		},
		"WithDebugLoggingEnabled": {
			input: map[string]interface{}{
				"agent": map[string]interface{}{
					"debug": true,
				},
			},
			want: confmap.NewFromStringMap(map[string]any{
				"verbosity": "detailed",
			}),
		},
	}
	factory := debugexporter.NewFactory()
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*debugexporter.Config)
				require.True(t, ok)
				wantCfg := factory.CreateDefaultConfig()
				require.NoError(t, component.UnmarshalConfig(testCase.want, wantCfg))
				assert.Equal(t, wantCfg, gotCfg)
			}
		})
	}
}
