// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlp

import (
	_ "embed"
	"fmt"
	"strconv"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	defaultGrpcEndpoint           = "127.0.0.1:4317"
	defaultHttpEndpoint           = "127.0.0.1:4318"
	defaultAppSignalsGrpcEndpoint = "0.0.0.0:4315"
	defaultAppSignalsHttpEndpoint = "0.0.0.0:4316"
	defaultJMXHttpEndpoint        = "0.0.0.0:4314"
)

// EndpointKey represents just the endpoint information for port conflict detection
type EndpointKey struct {
	GrpcEndpoint string
	HttpEndpoint string
}

// EndpointConfig represents the full configuration for registry comparison
type EndpointConfig struct {
	GrpcEndpoint string
	HttpEndpoint string
	TLSCertFile  string
	TLSKeyFile   string
}

func (e EndpointConfig) EndpointKey() EndpointKey {
	return EndpointKey{
		GrpcEndpoint: e.GrpcEndpoint,
		HttpEndpoint: e.HttpEndpoint,
	}
}

func (e EndpointConfig) IsJMX() bool {
	return e.GrpcEndpoint == ""
}

type translator struct {
	common.NameProvider
	common.IndexProvider
	config    EndpointConfig
	configKey string
	signal    pipeline.Signal
	factory   receiver.Factory
}

// WithSignal determines where the translator should look to find
// the configuration.
func WithSignal(signal pipeline.Signal) common.TranslatorOption {
	return func(target any) {
		if t, ok := target.(*translator); ok {
			t.signal = signal
		}
	}
}

func WithConfigKey(configKey string) common.TranslatorOption {
	return func(target any) {
		if t, ok := target.(*translator); ok {
			t.configKey = configKey
		}
	}
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator(opts ...common.TranslatorOption) common.ComponentTranslator {
	t := &translator{factory: otlpreceiver.NewFactory()}
	t.SetIndex(-1)
	for _, opt := range opts {
		opt(t)
	}
	if t.Name() == "" && t.signal.String() != "" {
		t.SetName(t.signal.String())
		if t.Index() != -1 {
			t.SetName(t.Name() + "/" + strconv.Itoa(t.Index()))
		}
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.Name())
}

var (
	registryMu sync.RWMutex
	registry   = make(map[EndpointConfig]common.ComponentTranslator)
)

func parseEndpointConfig(conf *confmap.Conf, configKey string, index int, signal pipeline.Signal, name string) EndpointConfig {
	config := EndpointConfig{
		GrpcEndpoint: defaultGrpcEndpoint,
		HttpEndpoint: defaultHttpEndpoint,
	}

	// Handle JMX special case
	if name == common.PipelineNameJmx {
		config.GrpcEndpoint = ""
		config.HttpEndpoint = defaultJMXHttpEndpoint
		return config
	}

	// Handle app signals special case
	if name == common.AppSignals {
		config.GrpcEndpoint = defaultAppSignalsGrpcEndpoint
		config.HttpEndpoint = defaultAppSignalsHttpEndpoint
		return config
	}

	if conf != nil && configKey != "" && conf.IsSet(configKey) {
		otlpMap := common.GetIndexedMap(conf, configKey, index)
		if grpc, ok := otlpMap["grpc_endpoint"]; ok {
			config.GrpcEndpoint = grpc.(string)
		}
		if http, ok := otlpMap["http_endpoint"]; ok {
			config.HttpEndpoint = http.(string)
		}
		if tls, ok := otlpMap["tls"].(map[string]interface{}); ok {
			if cert, ok := tls["cert_file"].(string); ok {
				config.TLSCertFile = cert
			}
			if key, ok := tls["key_file"].(string); ok {
				config.TLSKeyFile = key
			}
		}
	}

	return config
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*otlpreceiver.Config)

	// If we have a pre-computed config (from registry), use it
	if t.config.GrpcEndpoint != "" || t.config.HttpEndpoint != "" {
		return t.applyConfig(cfg, t.config), nil
	}

	// For NewTranslator, parse configuration using existing parseEndpointConfig
	var config EndpointConfig
	if t.configKey != "" && conf.IsSet(t.configKey) {
		config = parseEndpointConfig(conf, t.configKey, t.Index(), t.signal, t.Name())
	} else if t.Name() == common.PipelineNameJmx {
		config = parseEndpointConfig(conf, "", t.Index(), t.signal, t.Name())
	} else if t.Name() == common.AppSignals {
		config = parseEndpointConfig(conf, "", t.Index(), t.signal, t.Name())
		// Handle app signals TLS parsing
		if conf != nil {
			appSignalsConfigKeys, ok := common.AppSignalsConfigKeys[t.signal]
			if ok {
				for _, appKey := range appSignalsConfigKeys {
					if conf.IsSet(appKey) {
						otlpMap := common.GetIndexedMap(conf, appKey, t.Index())
						if tls, ok := otlpMap["tls"].(map[string]interface{}); ok {
							if cert, ok := tls["cert_file"].(string); ok {
								config.TLSCertFile = cert
							}
							if key, ok := tls["key_file"].(string); ok {
								config.TLSKeyFile = key
							}
						}
						break
					}
				}
			}
		}
	} else if t.configKey == "" {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: "missing config key"}
	} else {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: t.configKey}
	}

	// Check registry for sharing and port conflicts
	registryMu.Lock()
	defer registryMu.Unlock()

	// Check if we already have this exact component configuration
	if existing, exists := registry[config]; exists {
		// Update this translator to use the shared config and name
		t.config = config
		t.SetName(existing.(*translator).Name())
		return existing.Translate(conf)
	}

	// Check for port conflicts using EndpointKey
	endpointKey := config.EndpointKey()
	for existingConfig := range registry {
		if existingConfig.EndpointKey() == endpointKey {
			return nil, fmt.Errorf("port conflict: endpoints %s|%s already in use with different configuration", endpointKey.GrpcEndpoint, endpointKey.HttpEndpoint)
		}
	}

	// Register this translator with the parsed config
	t.config = config
	t.SetName(fmt.Sprintf("otlp_%d", len(registry)))
	registry[config] = t

	return t.applyConfig(cfg, config), nil
}

func (t *translator) applyConfig(cfg *otlpreceiver.Config, config EndpointConfig) *otlpreceiver.Config {
	if config.IsJMX() {
		cfg.GRPC = nil
		cfg.HTTP.ServerConfig.Endpoint = config.HttpEndpoint
		return cfg
	}

	cfg.GRPC.NetAddr.Endpoint = config.GrpcEndpoint
	cfg.HTTP.ServerConfig.Endpoint = config.HttpEndpoint

	if config.TLSCertFile != "" && config.TLSKeyFile != "" {
		tlsSettings := &configtls.ServerConfig{
			Config: configtls.Config{
				CertFile: config.TLSCertFile,
				KeyFile:  config.TLSKeyFile,
			},
		}
		cfg.GRPC.TLSSetting = tlsSettings
		cfg.HTTP.ServerConfig.TLSSetting = tlsSettings
	}

	return cfg
}
