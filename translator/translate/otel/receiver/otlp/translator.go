// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlp

import (
	"fmt"
	"strings"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type protocol string

const (
	HTTP protocol = "http"
	GRPC protocol = "grpc"

	defaultGrpcEndpoint           = "127.0.0.1:4317"
	defaultHttpEndpoint           = "127.0.0.1:4318"
	defaultAppSignalsGrpcEndpoint = "0.0.0.0:4315"
	defaultAppSignalsHttpEndpoint = "0.0.0.0:4316"
	defaultJMXHttpEndpoint        = "0.0.0.0:4314"
)

type translator struct {
	common.NameProvider
	common.IndexProvider
	signal  pipeline.Signal
	factory receiver.Factory
	cfg     component.Config
	err     error
}

type EndpointConfig struct {
	protocol protocol
	endpoint string
	certFile string
	keyFile  string
}

var (
	configCache = make(map[EndpointConfig]component.Config)
	cacheMutex  sync.RWMutex
)

func WithSignal(signal pipeline.Signal) common.TranslatorOption {
	return func(target any) {
		if t, ok := target.(*translator); ok {
			t.signal = signal
		}
	}
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator(otlpConfig EndpointConfig, opts ...common.TranslatorOption) common.ComponentTranslator {
	t := &translator{factory: otlpreceiver.NewFactory()}
	t.SetIndex(-1)
	for _, opt := range opts {
		opt(t)
	}

	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	// set name as "{type - http or grpc}" then appends "_{port}" if available
	t.SetName(string(otlpConfig.protocol))
	if parts := strings.Split(otlpConfig.endpoint, ":"); len(parts) > 1 {
		t.SetName(t.Name() + "_" + parts[1])
	}

	// check and get existing receiver config in the cache
	if existingCfg, exists := configCache[otlpConfig]; exists {
		t.cfg = existingCfg
		return t
	}

	// check for conflicting TLS configurations
	for cachedConfig := range configCache {
		if cachedConfig.protocol == otlpConfig.protocol && cachedConfig.endpoint == otlpConfig.endpoint &&
			(cachedConfig.certFile != otlpConfig.certFile || cachedConfig.keyFile != otlpConfig.keyFile) {
			t.err = fmt.Errorf("conflicting TLS configuration for %s endpoint %s", otlpConfig.protocol, otlpConfig.endpoint)
			return t
		}
	}

	cfg := t.factory.CreateDefaultConfig().(*otlpreceiver.Config)

	tlsSettings := &configtls.ServerConfig{}
	if otlpConfig.certFile != "" || otlpConfig.keyFile != "" {
		tlsSettings.CertFile = otlpConfig.certFile
		tlsSettings.KeyFile = otlpConfig.keyFile
	}

	if otlpConfig.protocol == HTTP {
		cfg.GRPC = nil
		cfg.HTTP.ServerConfig.Endpoint = otlpConfig.endpoint
		cfg.HTTP.ServerConfig.TLSSetting = tlsSettings
	} else {
		cfg.HTTP = nil
		cfg.GRPC.NetAddr.Endpoint = otlpConfig.endpoint
		cfg.GRPC.TLSSetting = tlsSettings
	}

	configCache[otlpConfig] = cfg
	t.cfg = cfg
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.Name())
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	return t.cfg, t.err
}

func ParseOtlpConfig(conf *confmap.Conf, pipelineName string, configKey string, signal pipeline.Signal, index int) ([]EndpointConfig, error) {
	if conf == nil {
		return nil, nil
	}

	// JMX only supports HTTP
	if pipelineName == common.PipelineNameJmx {
		return []EndpointConfig{{protocol: HTTP, endpoint: defaultJMXHttpEndpoint}}, nil
	}

	grpcDefault := defaultGrpcEndpoint
	httpDefault := defaultHttpEndpoint

	if pipelineName == common.AppSignals {
		appSignalsKeys := common.AppSignalsConfigKeys[signal]
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
		return []EndpointConfig{
			{protocol: GRPC, endpoint: grpcDefault},
			{protocol: HTTP, endpoint: httpDefault},
		}, nil
	}

	// Parse config
	otlpMap := common.GetIndexedMap(conf, configKey, index)
	var certFile, keyFile string
	if tls, ok := otlpMap["tls"].(map[string]interface{}); ok {
		certFile, _ = tls["cert_file"].(string)
		keyFile, _ = tls["key_file"].(string)
	}

	var configs []EndpointConfig
	if grpcEndpoint, ok := otlpMap["grpc_endpoint"].(string); ok {
		configs = append(configs, EndpointConfig{
			protocol: GRPC, endpoint: grpcEndpoint, certFile: certFile, keyFile: keyFile,
		})
	}
	if httpEndpoint, ok := otlpMap["http_endpoint"].(string); ok {
		configs = append(configs, EndpointConfig{
			protocol: HTTP, endpoint: httpEndpoint, certFile: certFile, keyFile: keyFile,
		})
	}

	// If no specific endpoints configured, return defaults
	if len(configs) == 0 {
		configs = append(configs, EndpointConfig{
			protocol: GRPC, endpoint: grpcDefault, certFile: certFile, keyFile: keyFile,
		})
		configs = append(configs, EndpointConfig{
			protocol: HTTP, endpoint: httpDefault, certFile: certFile, keyFile: keyFile,
		})
	}

	return configs, nil
}
