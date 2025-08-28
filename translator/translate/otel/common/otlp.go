package common

import (
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"
)

type OtlpProtocol string

const (
	HTTP OtlpProtocol = "http"
	GRPC OtlpProtocol = "grpc"

	defaultGrpcEndpoint           = "127.0.0.1:4317"
	defaultHttpEndpoint           = "127.0.0.1:4318"
	defaultAppSignalsGrpcEndpoint = "0.0.0.0:4315"
	defaultAppSignalsHttpEndpoint = "0.0.0.0:4316"
	defaultJMXHttpEndpoint        = "0.0.0.0:4314"
)

type OtlpEndpointConfig struct {
	Protocol OtlpProtocol
	Endpoint string
	CertFile string
	KeyFile  string
}

func ParseOtlpConfig(conf *confmap.Conf, pipelineName string, configKey string, signal pipeline.Signal, index int) ([]OtlpEndpointConfig, error) {
	if conf == nil {
		return nil, nil
	}

	// JMX only supports HTTP
	if pipelineName == PipelineNameJmx {
		return []OtlpEndpointConfig{{Protocol: HTTP, Endpoint: defaultJMXHttpEndpoint}}, nil
	}

	grpcDefault := defaultGrpcEndpoint
	httpDefault := defaultHttpEndpoint

	if pipelineName == AppSignals {
		appSignalsKeys := AppSignalsConfigKeys[signal]
		if conf.IsSet(appSignalsKeys[0]) {
			configKey = appSignalsKeys[0]
		} else {
			configKey = appSignalsKeys[1]
		}
		grpcDefault = defaultAppSignalsGrpcEndpoint
		httpDefault = defaultAppSignalsHttpEndpoint
	}

	// Use defaults if no config
	if !conf.IsSet(configKey) {
		return []OtlpEndpointConfig{
			{Protocol: GRPC, Endpoint: grpcDefault},
			{Protocol: HTTP, Endpoint: httpDefault},
		}, nil
	}

	// Parse config
	otlpMap := GetIndexedMap(conf, configKey, index)
	var certFile, keyFile string
	if tls, ok := otlpMap["tls"].(map[string]interface{}); ok {
		certFile, _ = tls["cert_file"].(string)
		keyFile, _ = tls["key_file"].(string)
	}

	var configs []OtlpEndpointConfig
	if grpcEndpoint, ok := otlpMap["grpc_endpoint"].(string); ok {
		configs = append(configs, OtlpEndpointConfig{
			Protocol: GRPC, Endpoint: grpcEndpoint, CertFile: certFile, KeyFile: keyFile,
		})
	}
	if httpEndpoint, ok := otlpMap["http_endpoint"].(string); ok {
		configs = append(configs, OtlpEndpointConfig{
			Protocol: HTTP, Endpoint: httpEndpoint, CertFile: certFile, KeyFile: keyFile,
		})
	}

	// If no specific endpoints configured, return defaults
	if len(configs) == 0 {
		configs = append(configs, OtlpEndpointConfig{
			Protocol: GRPC, Endpoint: grpcDefault, CertFile: certFile, KeyFile: keyFile,
		})
		configs = append(configs, OtlpEndpointConfig{
			Protocol: HTTP, Endpoint: httpDefault, CertFile: certFile, KeyFile: keyFile,
		})
	}

	return configs, nil
}
