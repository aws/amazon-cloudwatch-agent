// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslatorWithoutDataType(t *testing.T) {
	config := common.OtlpEndpointConfig{
		Protocol: common.HTTP,
		Endpoint: "127.0.0.1:4318",
	}
	tt := NewTranslator(config)
	assert.Contains(t, tt.ID().String(), "otlp")
	got, err := tt.Translate(confmap.New())
	assert.NoError(t, err)
	assert.NotNil(t, got)
}

func TestTracesTranslator(t *testing.T) {
	testCases := map[string]struct {
		config  common.OtlpEndpointConfig
		want    func(*otlpreceiver.Config) bool
		wantErr error
	}{
		"WithGRPCDefault": {
			config: common.OtlpEndpointConfig{
				Protocol: common.GRPC,
				Endpoint: "127.0.0.1:4317",
			},
			want: func(cfg *otlpreceiver.Config) bool {
				return cfg.GRPC != nil && cfg.GRPC.NetAddr.Endpoint == "127.0.0.1:4317" && cfg.HTTP == nil
			},
		},
		"WithHTTPDefault": {
			config: common.OtlpEndpointConfig{
				Protocol: common.HTTP,
				Endpoint: "127.0.0.1:4318",
			},
			want: func(cfg *otlpreceiver.Config) bool {
				return cfg.HTTP != nil && cfg.HTTP.ServerConfig.Endpoint == "127.0.0.1:4318" && cfg.GRPC == nil
			},
		},
		"WithTLS": {
			config: common.OtlpEndpointConfig{
				Protocol: common.GRPC,
				Endpoint: "127.0.0.1:4317",
				CertFile: "path/to/cert.crt",
				KeyFile:  "path/to/key.key",
			},
			want: func(cfg *otlpreceiver.Config) bool {
				return cfg.GRPC != nil &&
					cfg.GRPC.TLSSetting != nil &&
					cfg.GRPC.TLSSetting.CertFile == "path/to/cert.crt" &&
					cfg.GRPC.TLSSetting.KeyFile == "path/to/key.key"
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			tt := NewTranslator(testCase.config, WithSignal(pipeline.SignalTraces))
			got, err := tt.Translate(confmap.New())
			assert.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*otlpreceiver.Config)
				require.True(t, ok)
				assert.True(t, testCase.want(gotCfg))
			}
		})
	}
}

func TestMetricsTranslator(t *testing.T) {
	testCases := map[string]struct {
		config  common.OtlpEndpointConfig
		want    func(*otlpreceiver.Config) bool
		wantErr error
	}{
		"WithGRPCEndpoint": {
			config: common.OtlpEndpointConfig{
				Protocol: common.GRPC,
				Endpoint: "127.0.0.1:1234",
			},
			want: func(cfg *otlpreceiver.Config) bool {
				return cfg.GRPC != nil && cfg.GRPC.NetAddr.Endpoint == "127.0.0.1:1234"
			},
		},
		"WithHTTPEndpoint": {
			config: common.OtlpEndpointConfig{
				Protocol: common.HTTP,
				Endpoint: "127.0.0.1:2345",
			},
			want: func(cfg *otlpreceiver.Config) bool {
				return cfg.HTTP != nil && cfg.HTTP.ServerConfig.Endpoint == "127.0.0.1:2345"
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			tt := NewTranslator(testCase.config, WithSignal(pipeline.SignalMetrics))
			got, err := tt.Translate(confmap.New())
			assert.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*otlpreceiver.Config)
				require.True(t, ok)
				assert.True(t, testCase.want(gotCfg))
			}
		})
	}
}

func TestCaching(t *testing.T) {
	config := common.OtlpEndpointConfig{
		Protocol: common.HTTP,
		Endpoint: "127.0.0.1:4318",
	}

	tt1 := NewTranslator(config)
	tt2 := NewTranslator(config)

	cfg1, err1 := tt1.Translate(confmap.New())
	cfg2, err2 := tt2.Translate(confmap.New())

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.Equal(t, cfg1, cfg2) // Should be the same cached config
}
