// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package profiles

import (
	"fmt"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"
	"go.opentelemetry.io/collector/featuregate"
	_ "go.opentelemetry.io/collector/service/pipelines"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/otlp"
)

func TestTranslator(t *testing.T) {
	type want struct {
		receivers  []string
		processors []string
		exporters  []string
		extensions []string
	}
	tt := NewTranslator()
	assert.EqualValues(t, "profiles/otlp", tt.ID().String())
	testCases := map[string]struct {
		input   map[string]interface{}
		region  string
		want    *want
		wantErr error
	}{
		"WithoutOtlpKey": {
			input:   map[string]interface{}{},
			region:  "us-east-1",
			wantErr: &common.MissingKeyError{ID: tt.ID(), JsonKey: otlpKey},
		},
		"WithoutRegion": {
			input: map[string]interface{}{
				"opentelemetry": map[string]interface{}{
					"collect": map[string]interface{}{
						"otlp": map[string]interface{}{},
					},
				},
			},
			wantErr: fmt.Errorf("region is required for the profiles pipeline"),
		},
		"WithDefaults": {
			input: map[string]interface{}{
				"opentelemetry": map[string]interface{}{
					"collect": map[string]interface{}{
						"otlp": map[string]interface{}{},
					},
				},
			},
			region: "us-east-1",
			want: &want{
				receivers:  []string{"otlp/grpc_127_0_0_1_4317", "otlp/http_127_0_0_1_4318"},
				processors: []string{"resource/profiles"},
				exporters:  []string{"otlp_http/profiles"},
				extensions: []string{"sigv4auth/monitoring", "agenthealth/profiles"},
			},
		},
		"WithEndpoints": {
			input: map[string]interface{}{
				"opentelemetry": map[string]interface{}{
					"collect": map[string]interface{}{
						"otlp": map[string]interface{}{
							"grpc_endpoint": "127.0.0.1:5317",
							"http_endpoint": "127.0.0.1:5318",
						},
					},
				},
			},
			region: "us-east-1",
			want: &want{
				receivers:  []string{"otlp/grpc_127_0_0_1_5317", "otlp/http_127_0_0_1_5318"},
				processors: []string{"resource/profiles"},
				exporters:  []string{"otlp_http/profiles"},
				extensions: []string{"sigv4auth/monitoring", "agenthealth/profiles"},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Cleanup(otlp.ClearConfigCache)
			resetGlobalConfig(t, testCase.region)
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
				for _, id := range got.Processors.Keys() {
					assert.NotEqual(t, "batch", id.Type().String())
				}
			}
		})
	}
}

func TestExporterEndpoint(t *testing.T) {
	testCases := map[string]struct {
		region string
		want   string
	}{
		"WithRegion": {
			region: "us-east-1",
			want:   "https://monitoring.us-east-1.amazonaws.com/v1development/profiles",
		},
		"WithChinaRegion": {
			region: "cn-north-1",
			want:   "https://monitoring.cn-north-1.amazonaws.com.cn/v1development/profiles",
		},
		"WithGovCloudRegion": {
			region: "us-gov-west-1",
			want:   "https://monitoring.us-gov-west-1.amazonaws.com/v1development/profiles",
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Cleanup(otlp.ClearConfigCache)
			resetGlobalConfig(t, testCase.region)
			conf := confmap.NewFromStringMap(map[string]interface{}{
				"opentelemetry": map[string]interface{}{
					"collect": map[string]interface{}{
						"otlp": map[string]interface{}{},
					},
				},
			})
			got, err := NewTranslator().Translate(conf)
			require.NoError(t, err)
			exporterTranslator, ok := got.Exporters.Get(component.MustNewIDWithName("otlp_http", "profiles"))
			require.True(t, ok)
			cfg, err := exporterTranslator.Translate(conf)
			require.NoError(t, err)
			exporterCfg, ok := cfg.(*otlphttpexporter.Config)
			require.True(t, ok)
			assert.Equal(t, testCase.want, exporterCfg.ProfilesEndpoint)
			assert.EqualValues(t, "gzip", exporterCfg.ClientConfig.Compression)
			require.True(t, exporterCfg.ClientConfig.Auth.HasValue())
			assert.Equal(t, "agenthealth/profiles", exporterCfg.ClientConfig.Auth.Get().AuthenticatorID.String())
		})
	}
}

func TestServiceNameFallbackStamped(t *testing.T) {
	t.Cleanup(otlp.ClearConfigCache)
	resetGlobalConfig(t, "us-east-1")
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"otlp": map[string]interface{}{},
			},
		},
	})
	got, err := NewTranslator().Translate(conf)
	require.NoError(t, err)
	processorTranslator, ok := got.Processors.Get(component.MustNewIDWithName("resource", "profiles"))
	require.True(t, ok)
	cfg, err := processorTranslator.Translate(conf)
	require.NoError(t, err)
	processorCfg, ok := cfg.(*resourceprocessor.Config)
	require.True(t, ok)
	require.Len(t, processorCfg.AttributesActions, 1)
	action := processorCfg.AttributesActions[0]
	assert.Equal(t, "service.name", action.Key)
	assert.EqualValues(t, "insert", action.Action)
	assert.Equal(t, "unknown_service", action.Value)
}

func TestProfilesSupportGateEnabledByInit(t *testing.T) {
	assert.True(t, gateEnabled(t))
}

func gateEnabled(t *testing.T) bool {
	t.Helper()
	var enabled bool
	featuregate.GlobalRegistry().VisitAll(func(gate *featuregate.Gate) {
		if gate.ID() == profilesSupportGateID {
			enabled = gate.IsEnabled()
		}
	})
	return enabled
}

func resetGlobalConfig(t *testing.T, region string) {
	t.Helper()
	previous := agent.Global_Config
	t.Cleanup(func() {
		agent.Global_Config = previous
	})
	agent.Global_Config = agent.Agent{Region: region}
}
