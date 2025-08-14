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

type translator struct {
	common.NameProvider
	common.IndexProvider
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
	registry   = make(map[string]common.ComponentTranslator)
	counter    int
)

func getEndpointKey(conf *confmap.Conf, configKey string, index int, signal pipeline.Signal) (string, error) {
	grpcEndpoint := defaultGrpcEndpoint
	httpEndpoint := defaultHttpEndpoint
	tlsCert, tlsKey := "", ""

	// Handle application signals defaults
	if appSignalsConfigKeys, ok := common.AppSignalsConfigKeys[signal]; ok {
		for _, appKey := range appSignalsConfigKeys {
			if configKey == appKey {
				grpcEndpoint = defaultAppSignalsGrpcEndpoint
				httpEndpoint = defaultAppSignalsHttpEndpoint
				break
			}
		}
	}

	if conf != nil && conf.IsSet(configKey) {
		otlpMap := common.GetIndexedMap(conf, configKey, index)
		if grpc, ok := otlpMap["grpc_endpoint"]; ok {
			grpcEndpoint = grpc.(string)
		}
		if http, ok := otlpMap["http_endpoint"]; ok {
			httpEndpoint = http.(string)
		}
		if tls, ok := otlpMap["tls"].(map[string]interface{}); ok {
			if cert, ok := tls["cert_file"].(string); ok {
				tlsCert = cert
			}
			if key, ok := tls["key_file"].(string); ok {
				tlsKey = key
			}
		}
	}

	portKey := fmt.Sprintf("%s|%s", grpcEndpoint, httpEndpoint)
	fullKey := fmt.Sprintf("%s|%s|%s", portKey, tlsCert, tlsKey)

	// Check for port conflicts with different TLS configs
	for existingKey := range registry {
		if existingKey != fullKey && existingKey[:len(portKey)] == portKey && len(existingKey) > len(portKey) {
			return "", fmt.Errorf("port conflict: endpoints %s already in use with different TLS configuration", portKey)
		}
	}

	return fullKey, nil
}

func NewSharedTranslator(conf *confmap.Conf, opts ...common.TranslatorOption) common.ComponentTranslator {
	t := &translator{factory: otlpreceiver.NewFactory()}
	t.SetIndex(-1)
	for _, opt := range opts {
		opt(t)
	}

	registryMu.Lock()
	defer registryMu.Unlock()

	key, err := getEndpointKey(conf, t.configKey, t.Index(), t.signal)
	if err != nil {
		// Return error translator that will fail during Translate()
		return &errorTranslator{err: err}
	}

	if existing, exists := registry[key]; exists {
		return existing
	}

	counter++
	t.SetName(fmt.Sprintf("otlp_shared_%d", counter))
	registry[key] = t
	return t
}

type errorTranslator struct {
	err error
}

func (e *errorTranslator) ID() component.ID {
	return component.NewID(component.MustNewType("otlp"))
}

func (e *errorTranslator) Translate(*confmap.Conf) (component.Config, error) {
	return nil, e.err
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*otlpreceiver.Config)

	if t.Name() == common.PipelineNameJmx {
		cfg.GRPC = nil
		cfg.HTTP.ServerConfig.Endpoint = defaultJMXHttpEndpoint
		return cfg, nil
	}

	// init default configuration
	configKey := t.configKey
	cfg.GRPC.NetAddr.Endpoint = defaultGrpcEndpoint
	cfg.HTTP.ServerConfig.Endpoint = defaultHttpEndpoint

	if t.Name() == common.AppSignals {
		appSignalsConfigKeys, ok := common.AppSignalsConfigKeys[t.signal]
		if !ok {
			return nil, fmt.Errorf("no application_signals config key defined for signal: %s", t.signal)
		}
		if conf.IsSet(appSignalsConfigKeys[0]) {
			configKey = appSignalsConfigKeys[0]
		} else {
			configKey = appSignalsConfigKeys[1]
		}
		cfg.GRPC.NetAddr.Endpoint = defaultAppSignalsGrpcEndpoint
		cfg.HTTP.ServerConfig.Endpoint = defaultAppSignalsHttpEndpoint
	}

	if conf == nil || !conf.IsSet(configKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: configKey}
	}

	otlpMap := common.GetIndexedMap(conf, configKey, t.Index())
	var tlsSettings *configtls.ServerConfig
	if tls, ok := otlpMap["tls"].(map[string]interface{}); ok {
		tlsSettings = &configtls.ServerConfig{}
		tlsSettings.CertFile = tls["cert_file"].(string)
		tlsSettings.KeyFile = tls["key_file"].(string)
	}
	cfg.GRPC.TLSSetting = tlsSettings
	cfg.HTTP.ServerConfig.TLSSetting = tlsSettings

	grpcEndpoint, grpcOk := otlpMap["grpc_endpoint"]
	httpEndpoint, httpOk := otlpMap["http_endpoint"]
	if grpcOk {
		cfg.GRPC.NetAddr.Endpoint = grpcEndpoint.(string)
	}
	if httpOk {
		cfg.HTTP.ServerConfig.Endpoint = httpEndpoint.(string)
	}
	return cfg, nil
}

// ResetRegistry clears the shared receiver registry for testing
func ResetRegistry() {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = make(map[string]common.ComponentTranslator)
	counter = 0
}
