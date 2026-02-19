// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlphttpexporter

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
)

func TestTranslator(t *testing.T) {
	agent.Global_Config.Region = "us-east-1"
	agent.Global_Config.Role_arn = "global_arn"
	tt := NewTranslator()
	assert.EqualValues(t, "otlphttp", tt.ID().String())
	testCases := map[string]struct {
		input   map[string]any
		want    *confmap.Conf
		wantErr error
	}{
		"WithMissingEndpoint": {
			input:   map[string]any{"metrics": map[string]any{}},
			wantErr: errors.New("otlphttpexporter: missing required endpoint"),
		},
		"WithEndpoint": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_destinations": map[string]any{
						"otlp": map[string]any{
							"endpoint": "https://custom-endpoint.com/v1/metrics",
						},
					},
				},
			},
			want: confmap.NewFromStringMap(map[string]any{
				"metrics_endpoint": "https://custom-endpoint.com/v1/metrics",
				"auth": map[string]any{
					"authenticator": "sigv4auth/monitoring",
				},
			}),
		},
	}
	factory := otlphttpexporter.NewFactory()
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*otlphttpexporter.Config)
				require.True(t, ok)
				wantCfg := factory.CreateDefaultConfig()
				require.NoError(t, testCase.want.Unmarshal(wantCfg))
				assert.Equal(t, wantCfg, gotCfg)
			}
		})
	}
}
