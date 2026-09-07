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
	assert.EqualValues(t, "profiles/profiles", tt.ID().String())
	testCases := map[string]struct {
		input   map[string]interface{}
		region  string
		want    *want
		wantErr error
	}{
		"WithoutProfilesKey": {
			input:   map[string]interface{}{},
			region:  "us-east-1",
			wantErr: &common.MissingKeyError{ID: tt.ID(), JsonKey: common.ProfilesKey},
		},
		"WithoutServiceName": {
			input: map[string]interface{}{
				"profiles": map[string]interface{}{},
			},
			region:  "us-east-1",
			wantErr: fmt.Errorf("%q is required for the profiles pipeline", serviceNameKey),
		},
		"WithBlankServiceName": {
			input: map[string]interface{}{
				"profiles": map[string]interface{}{
					"service_name": "  ",
				},
			},
			region:  "us-east-1",
			wantErr: fmt.Errorf("%q is required for the profiles pipeline", serviceNameKey),
		},
		"WithoutRegion": {
			input: map[string]interface{}{
				"profiles": map[string]interface{}{
					"service_name": "my-service",
				},
			},
			wantErr: fmt.Errorf("region is required for the profiles pipeline"),
		},
		"WithDefaults": {
			input: map[string]interface{}{
				"profiles": map[string]interface{}{
					"service_name": "my-service",
				},
			},
			region: "us-east-1",
			want: &want{
				receivers:  []string{"otlp/http_127_0_0_1_4318"},
				processors: []string{"resource/profiles"},
				exporters:  []string{"otlp_http/profiles"},
				extensions: []string{"sigv4auth/monitoring", "agenthealth/profiles"},
			},
		},
		"WithHTTPEndpoint": {
			input: map[string]interface{}{
				"profiles": map[string]interface{}{
					"service_name": "my-service",
					"otlp": map[string]interface{}{
						"http_endpoint": "0.0.0.0:5318",
					},
				},
			},
			region: "us-east-1",
			want: &want{
				receivers:  []string{"otlp/http_0_0_0_0_5318"},
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
		profiles map[string]interface{}
		region   string
		want     string
	}{
		"WithRegion": {
			profiles: map[string]interface{}{
				"service_name": "my-service",
			},
			region: "us-east-1",
			want:   "https://monitoring.us-east-1.amazonaws.com/v1development/profiles",
		},
		"WithChinaRegion": {
			profiles: map[string]interface{}{
				"service_name": "my-service",
			},
			region: "cn-north-1",
			want:   "https://monitoring.cn-north-1.amazonaws.com.cn/v1development/profiles",
		},
		"WithGovCloudRegion": {
			profiles: map[string]interface{}{
				"service_name": "my-service",
			},
			region: "us-gov-west-1",
			want:   "https://monitoring.us-gov-west-1.amazonaws.com/v1development/profiles",
		},
		"WithEndpointOverride": {
			profiles: map[string]interface{}{
				"service_name":      "my-service",
				"endpoint_override": "monitoring-fips.us-east-1.amazonaws.com",
			},
			region: "us-east-1",
			want:   "https://monitoring-fips.us-east-1.amazonaws.com/v1development/profiles",
		},
		"WithEndpointOverrideSchemeAndTrailingSlash": {
			profiles: map[string]interface{}{
				"service_name":      "my-service",
				"endpoint_override": "https://example.us-east-1.amazonaws.com/",
			},
			region: "us-east-1",
			want:   "https://example.us-east-1.amazonaws.com/v1development/profiles",
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Cleanup(otlp.ClearConfigCache)
			resetGlobalConfig(t, testCase.region)
			conf := confmap.NewFromStringMap(map[string]interface{}{"profiles": testCase.profiles})
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

func TestServiceNameStamped(t *testing.T) {
	t.Cleanup(otlp.ClearConfigCache)
	resetGlobalConfig(t, "us-east-1")
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"profiles": map[string]interface{}{
			"service_name": " my-service ",
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
	assert.EqualValues(t, "upsert", action.Action)
	assert.Equal(t, "my-service", action.Value)
}

func TestEnablesProfilesSupportGate(t *testing.T) {
	t.Cleanup(otlp.ClearConfigCache)
	resetGlobalConfig(t, "us-east-1")
	previous := gateEnabled(t)
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(profilesSupportGateID, previous))
	})
	require.NoError(t, featuregate.GlobalRegistry().Set(profilesSupportGateID, false))
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"profiles": map[string]interface{}{
			"service_name": "my-service",
		},
	})
	_, err := NewTranslator().Translate(conf)
	require.NoError(t, err)
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
