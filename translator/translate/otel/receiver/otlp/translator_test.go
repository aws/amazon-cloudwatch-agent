// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslatorWithoutDataType(t *testing.T) {
	config := EndpointConfig{
		protocol: http,
		endpoint: "127.0.0.1:4318",
	}
	tt := NewTranslator(config)
	assert.Contains(t, tt.ID().String(), "otlp")
	got, err := tt.Translate(confmap.New())
	assert.NoError(t, err)
	assert.NotNil(t, got)
}

func TestTracesTranslator(t *testing.T) {
	// Clear cache before test
	configCache = make(map[EndpointConfig]component.Config)

	testCases := map[string]struct {
		config  EndpointConfig
		want    func(*otlpreceiver.Config) bool
		wantErr error
	}{
		"WithGRPCDefault": {
			config: EndpointConfig{
				protocol: grpc,
				endpoint: "127.0.0.1:4317",
			},
			want: func(cfg *otlpreceiver.Config) bool {
				return cfg.GRPC != nil && cfg.GRPC.NetAddr.Endpoint == "127.0.0.1:4317" && cfg.HTTP == nil
			},
		},
		"WithHTTPDefault": {
			config: EndpointConfig{
				protocol: http,
				endpoint: "127.0.0.1:4318",
			},
			want: func(cfg *otlpreceiver.Config) bool {
				return cfg.HTTP != nil && cfg.HTTP.ServerConfig.Endpoint == "127.0.0.1:4318" && cfg.GRPC == nil
			},
		},
		"WithTLS": {
			config: EndpointConfig{
				protocol: grpc,
				endpoint: "127.0.0.1:4317",
				certFile: "path/to/cert.crt",
				keyFile:  "path/to/key.key",
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
			// Clear cache before each test case
			configCache = make(map[EndpointConfig]component.Config)
			tt := NewTranslator(testCase.config)
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
	// Clear cache before test
	configCache = make(map[EndpointConfig]component.Config)

	testCases := map[string]struct {
		config  EndpointConfig
		want    func(*otlpreceiver.Config) bool
		wantErr error
	}{
		"WithGRPCEndpoint": {
			config: EndpointConfig{
				protocol: grpc,
				endpoint: "127.0.0.1:1234",
			},
			want: func(cfg *otlpreceiver.Config) bool {
				return cfg.GRPC != nil && cfg.GRPC.NetAddr.Endpoint == "127.0.0.1:1234"
			},
		},
		"WithHTTPEndpoint": {
			config: EndpointConfig{
				protocol: http,
				endpoint: "127.0.0.1:2345",
			},
			want: func(cfg *otlpreceiver.Config) bool {
				return cfg.HTTP != nil && cfg.HTTP.ServerConfig.Endpoint == "127.0.0.1:2345"
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Clear cache before each test case
			configCache = make(map[EndpointConfig]component.Config)
			tt := NewTranslator(testCase.config)
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
	ClearConfigCache()

	config := EndpointConfig{
		protocol: http,
		endpoint: "127.0.0.1:4318",
	}

	tt1 := NewTranslator(config)
	tt2 := NewTranslator(config)

	cfg1, err1 := tt1.Translate(confmap.New())
	cfg2, err2 := tt2.Translate(confmap.New())

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.Equal(t, cfg1, cfg2)
}

func TestTLSConflictDetection(t *testing.T) {
	ClearConfigCache()

	config1 := EndpointConfig{
		protocol: http,
		endpoint: "127.0.0.1:4318",
		certFile: "cert1.pem",
		keyFile:  "key1.pem",
	}
	tt1 := NewTranslator(config1)
	_, err1 := tt1.Translate(confmap.New())
	assert.NoError(t, err1)

	config2 := EndpointConfig{
		protocol: http,
		endpoint: "127.0.0.1:4318",
		certFile: "cert2.pem",
		keyFile:  "key2.pem",
	}
	tt2 := NewTranslator(config2)
	_, err2 := tt2.Translate(confmap.New())
	assert.Error(t, err2)
	assert.Contains(t, err2.Error(), "conflicting TLS configuration")
}

func TestAppSignalsTLSIgnore(t *testing.T) {
	t.Run("AllowsMixedTLSAndNoTLS", func(t *testing.T) {
		ClearConfigCache()

		// First config with TLS
		config1 := EndpointConfig{
			protocol: http,
			endpoint: "127.0.0.1:4318",
			certFile: "cert1.pem",
			keyFile:  "key1.pem",
		}
		tt1 := NewTranslator(config1, common.WithName(common.AppSignals))
		_, err1 := tt1.Translate(confmap.New())
		assert.NoError(t, err1)

		// Second config without TLS - should be allowed for AppSignals
		config2 := EndpointConfig{
			protocol: http,
			endpoint: "127.0.0.1:4318",
		}
		tt2 := NewTranslator(config2, common.WithName(common.AppSignals))
		_, err2 := tt2.Translate(confmap.New())
		assert.NoError(t, err2)
	})

	t.Run("AllowsNoTLSThenTLS", func(t *testing.T) {
		ClearConfigCache()

		// First config without TLS
		config1 := EndpointConfig{
			protocol: http,
			endpoint: "127.0.0.1:4318",
		}
		tt1 := NewTranslator(config1, common.WithName(common.AppSignals))
		_, err1 := tt1.Translate(confmap.New())
		assert.NoError(t, err1)

		// Second config with TLS - should be allowed for AppSignals
		config2 := EndpointConfig{
			protocol: http,
			endpoint: "127.0.0.1:4318",
			certFile: "cert1.pem",
			keyFile:  "key1.pem",
		}
		tt2 := NewTranslator(config2, common.WithName(common.AppSignals))
		_, err2 := tt2.Translate(confmap.New())
		assert.NoError(t, err2)
	})

	t.Run("ErrorsOnDifferentTLSConfigs", func(t *testing.T) {
		ClearConfigCache()

		// First config with TLS
		config1 := EndpointConfig{
			protocol: http,
			endpoint: "127.0.0.1:4318",
			certFile: "cert1.pem",
			keyFile:  "key1.pem",
		}
		tt1 := NewTranslator(config1, common.WithName(common.AppSignals))
		_, err1 := tt1.Translate(confmap.New())
		assert.NoError(t, err1)

		// Second config with different TLS - should error even for AppSignals
		config2 := EndpointConfig{
			protocol: http,
			endpoint: "127.0.0.1:4318",
			certFile: "cert2.pem",
			keyFile:  "key2.pem",
		}
		tt2 := NewTranslator(config2, common.WithName(common.AppSignals))
		_, err2 := tt2.Translate(confmap.New())
		assert.Error(t, err2)
		assert.Contains(t, err2.Error(), "conflicting TLS configuration")
	})

	t.Run("EmptyPipelineNameErrorsOnTLSConflict", func(t *testing.T) {
		ClearConfigCache()

		// First config with TLS
		config1 := EndpointConfig{
			protocol: http,
			endpoint: "127.0.0.1:4318",
			certFile: "cert1.pem",
			keyFile:  "key1.pem",
		}
		tt1 := NewTranslator(config1, common.WithName(""))
		_, err1 := tt1.Translate(confmap.New())
		assert.NoError(t, err1)

		// Second config without TLS - should error for empty pipeline name
		config2 := EndpointConfig{
			protocol: http,
			endpoint: "127.0.0.1:4318",
		}
		tt2 := NewTranslator(config2, common.WithName(""))
		_, err2 := tt2.Translate(confmap.New())
		assert.Error(t, err2)
		assert.Contains(t, err2.Error(), "conflicting TLS configuration")
	})
}

func TestTranslateToEndpointConfig_JMX(t *testing.T) {
	conf := confmap.New()
	configs := TranslateToEndpointConfig(conf, common.PipelineNameJmx, common.OtlpKey, -1)

	assert.Len(t, configs, 1)
	assert.Equal(t, http, configs[0].protocol)
	assert.Equal(t, "0.0.0.0:4314", configs[0].endpoint)
}

func TestTranslateToEndpointConfig_MissingKey(t *testing.T) {
	conf := confmap.New()
	configs := TranslateToEndpointConfig(conf, "test", "missing", -1)

	assert.Len(t, configs, 1)
	assert.Error(t, configs[0].err)
}

func TestTranslateToEndpointConfig_DefaultEndpoints(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"otlp": map[string]any{},
	})
	configs := TranslateToEndpointConfig(conf, "regular", "otlp", -1)

	assert.Len(t, configs, 2)
	assert.Equal(t, grpc, configs[0].protocol)
	assert.Equal(t, "127.0.0.1:4317", configs[0].endpoint)
	assert.Equal(t, http, configs[1].protocol)
	assert.Equal(t, "127.0.0.1:4318", configs[1].endpoint)
}

func TestTranslateToEndpointConfig_CustomEndpoints(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"otlp": map[string]any{
			"grpc_endpoint": "custom-grpc:4317",
			"http_endpoint": "custom-http:4318",
			"tls": map[string]any{
				"cert_file": "/path/to/cert",
				"key_file":  "/path/to/key",
			},
		},
	})

	configs := TranslateToEndpointConfig(conf, "regular", "otlp", -1)

	assert.Len(t, configs, 2)

	assert.Equal(t, grpc, configs[0].protocol)
	assert.Equal(t, "custom-grpc:4317", configs[0].endpoint)
	assert.Equal(t, "/path/to/cert", configs[0].certFile)
	assert.Equal(t, "/path/to/key", configs[0].keyFile)

	assert.Equal(t, http, configs[1].protocol)
	assert.Equal(t, "custom-http:4318", configs[1].endpoint)
	assert.Equal(t, "/path/to/cert", configs[1].certFile)
	assert.Equal(t, "/path/to/key", configs[1].keyFile)
}

func TestTranslateToEndpointConfig_OnlyGRPC(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"otlp": map[string]any{
			"grpc_endpoint": "grpc-only:4317",
		},
	})

	configs := TranslateToEndpointConfig(conf, "regular", "otlp", -1)

	assert.Len(t, configs, 1)
	assert.Equal(t, grpc, configs[0].protocol)
	assert.Equal(t, "grpc-only:4317", configs[0].endpoint)
}

func TestTranslateToEndpointConfig_OnlyHTTP(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"otlp": map[string]any{
			"http_endpoint": "http-only:4318",
		},
	})

	configs := TranslateToEndpointConfig(conf, "regular", "otlp", -1)

	assert.Len(t, configs, 1)
	assert.Equal(t, http, configs[0].protocol)
	assert.Equal(t, "http-only:4318", configs[0].endpoint)
}
