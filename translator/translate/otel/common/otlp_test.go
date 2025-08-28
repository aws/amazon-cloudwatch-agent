package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"
)

func TestParseOtlpConfig_NilConf(t *testing.T) {
	configs, err := ParseOtlpConfig(nil, "test", "otlp", pipeline.SignalTraces, -1)
	assert.NoError(t, err)
	assert.Nil(t, configs)
}

func TestParseOtlpConfig_JMX(t *testing.T) {
	conf := confmap.New()
	configs, err := ParseOtlpConfig(conf, PipelineNameJmx, OtlpKey, pipeline.SignalMetrics, -1)

	assert.NoError(t, err)
	assert.Len(t, configs, 1)
	assert.Equal(t, HTTP, configs[0].Protocol)
	assert.Equal(t, "0.0.0.0:4314", configs[0].Endpoint)
}

func TestParseOtlpConfig_AppSignals(t *testing.T) {
	conf := confmap.New()
	configs, err := ParseOtlpConfig(conf, AppSignals, "", pipeline.SignalTraces, -1)

	assert.NoError(t, err)
	assert.Len(t, configs, 2)
	assert.Equal(t, GRPC, configs[0].Protocol)
	assert.Equal(t, "0.0.0.0:4315", configs[0].Endpoint)
	assert.Equal(t, HTTP, configs[1].Protocol)
	assert.Equal(t, "0.0.0.0:4316", configs[1].Endpoint)
}

func TestParseOtlpConfig_DefaultEndpoints(t *testing.T) {
	conf := confmap.New()
	configs, err := ParseOtlpConfig(conf, "regular", OtlpKey, pipeline.SignalTraces, -1)

	assert.NoError(t, err)
	assert.Len(t, configs, 2)
	assert.Equal(t, GRPC, configs[0].Protocol)
	assert.Equal(t, "127.0.0.1:4317", configs[0].Endpoint)
	assert.Equal(t, HTTP, configs[1].Protocol)
	assert.Equal(t, "127.0.0.1:4318", configs[1].Endpoint)
}

func TestParseOtlpConfig_CustomEndpoints(t *testing.T) {
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

	configs, err := ParseOtlpConfig(conf, "regular", OtlpKey, pipeline.SignalTraces, -1)

	assert.NoError(t, err)
	assert.Len(t, configs, 2)

	assert.Equal(t, GRPC, configs[0].Protocol)
	assert.Equal(t, "custom-grpc:4317", configs[0].Endpoint)
	assert.Equal(t, "/path/to/cert", configs[0].CertFile)
	assert.Equal(t, "/path/to/key", configs[0].KeyFile)

	assert.Equal(t, HTTP, configs[1].Protocol)
	assert.Equal(t, "custom-http:4318", configs[1].Endpoint)
	assert.Equal(t, "/path/to/cert", configs[1].CertFile)
	assert.Equal(t, "/path/to/key", configs[1].KeyFile)
}

func TestParseOtlpConfig_OnlyGRPC(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"otlp": map[string]any{
			"grpc_endpoint": "grpc-only:4317",
		},
	})

	configs, err := ParseOtlpConfig(conf, "regular", OtlpKey, pipeline.SignalTraces, -1)

	assert.NoError(t, err)
	assert.Len(t, configs, 1)
	assert.Equal(t, GRPC, configs[0].Protocol)
	assert.Equal(t, "grpc-only:4317", configs[0].Endpoint)
}

func TestParseOtlpConfig_OnlyHTTP(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"otlp": map[string]any{
			"http_endpoint": "http-only:4318",
		},
	})

	configs, err := ParseOtlpConfig(conf, "regular", OtlpKey, pipeline.SignalTraces, -1)

	assert.NoError(t, err)
	assert.Len(t, configs, 1)
	assert.Equal(t, HTTP, configs[0].Protocol)
	assert.Equal(t, "http-only:4318", configs[0].Endpoint)
}
