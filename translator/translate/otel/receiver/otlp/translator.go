// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlp

import (
	"fmt"
	"regexp"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type protocol string

const (
	http protocol = "http"
	grpc protocol = "grpc"

	defaultGrpcEndpoint           = "127.0.0.1:4317"
	defaultHttpEndpoint           = "127.0.0.1:4318"
	defaultAppSignalsGrpcEndpoint = "0.0.0.0:4315"
	defaultAppSignalsHttpEndpoint = "0.0.0.0:4316"
	defaultJMXHttpEndpoint        = "0.0.0.0:4314"
)

type translator struct {
	common.NameProvider
	factory        receiver.Factory
	endpointConfig EndpointConfig
	pipelineName   string
}

type EndpointConfig struct {
	protocol protocol
	endpoint string
	certFile string
	keyFile  string
	err      error
}

var (
	configCache = make(map[EndpointConfig]component.Config)
	cacheMutex  sync.RWMutex
	epRegex     = regexp.MustCompile(`[^a-zA-Z0-9_]`)
)

// ClearConfigCache clears the OTLP config cache.
// this is intended for testing purposes only from in and out of package.
func ClearConfigCache() {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	configCache = make(map[EndpointConfig]component.Config)
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator(epConfig EndpointConfig, opts ...common.TranslatorOption) common.ComponentTranslator {
	t := &translator{
		factory:        otlpreceiver.NewFactory(),
		endpointConfig: epConfig,
	}
	for _, opt := range opts {
		opt(t)
	}

	t.pipelineName = t.Name()
	// set name as "{type - http or grpc}" then appends an endpoint by replacing special chars with '_' eg. 'http_0_0_0_0_4316'
	t.SetName(string(epConfig.protocol) + "_" + epRegex.ReplaceAllString(epConfig.endpoint, "_"))

	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.Name())
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	if t.endpointConfig.err != nil {
		return nil, t.endpointConfig.err
	}

	// check and get existing receiver config in the cache
	if existingCfg, exists := configCache[t.endpointConfig]; exists {
		return existingCfg, nil
	}

	for cachedConfig := range configCache {
		if cachedConfig.protocol == t.endpointConfig.protocol && cachedConfig.endpoint == t.endpointConfig.endpoint &&
			(cachedConfig.certFile != t.endpointConfig.certFile || cachedConfig.keyFile != t.endpointConfig.keyFile) {
			// ignores (missing) TLS conflict for application signals pipelines when one has TLS and the other doesn't
			// https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Agent-Application_Signals.html
			if t.pipelineName == common.AppSignals {
				cachedHasTLS := cachedConfig.certFile != "" || cachedConfig.keyFile != ""
				currentHasTLS := t.endpointConfig.certFile != "" || t.endpointConfig.keyFile != ""
				if cachedHasTLS != currentHasTLS {
					return configCache[cachedConfig], nil
				}
			}
			return nil, fmt.Errorf("conflicting TLS configuration for %s endpoint %s", t.endpointConfig.protocol, t.endpointConfig.endpoint)
		}
	}

	cfg := t.factory.CreateDefaultConfig().(*otlpreceiver.Config)

	tlsSettings := &configtls.ServerConfig{}
	if t.endpointConfig.certFile != "" || t.endpointConfig.keyFile != "" {
		tlsSettings.CertFile = t.endpointConfig.certFile
		tlsSettings.KeyFile = t.endpointConfig.keyFile
	}

	if t.endpointConfig.protocol == http {
		cfg.GRPC = nil
		cfg.HTTP.ServerConfig.Endpoint = t.endpointConfig.endpoint
		cfg.HTTP.ServerConfig.TLSSetting = tlsSettings
	} else {
		cfg.HTTP = nil
		cfg.GRPC.NetAddr.Endpoint = t.endpointConfig.endpoint
		cfg.GRPC.TLSSetting = tlsSettings
	}

	configCache[t.endpointConfig] = cfg
	return cfg, nil
}

func translateToEndpointConfig(conf *confmap.Conf, pipelineName string, configKey string, index int) []EndpointConfig {
	if pipelineName == common.PipelineNameJmx {
		return []EndpointConfig{{protocol: http, endpoint: defaultJMXHttpEndpoint}}
	}

	grpcDefault, httpDefault := defaultGrpcEndpoint, defaultHttpEndpoint

	if conf == nil || !conf.IsSet(configKey) {
		pipelineType, _ := component.NewType(pipelineName)
		return []EndpointConfig{{err: &common.MissingKeyError{ID: component.NewID(pipelineType), JsonKey: configKey}}}
	}

	if pipelineName == common.AppSignals {
		grpcDefault = defaultAppSignalsGrpcEndpoint
		httpDefault = defaultAppSignalsHttpEndpoint
	}

	otlpMap := common.GetIndexedMap(conf, configKey, index)
	certFile, keyFile := "", ""
	if tls, ok := otlpMap["tls"].(map[string]interface{}); ok {
		certFile, _ = tls["cert_file"].(string)
		keyFile, _ = tls["key_file"].(string)
	}

	var configs []EndpointConfig
	if grpcEndpoint, ok := otlpMap["grpc_endpoint"].(string); ok && grpcEndpoint != "" {
		configs = append(configs, EndpointConfig{grpc, grpcEndpoint, certFile, keyFile, nil})
	}
	if httpEndpoint, ok := otlpMap["http_endpoint"].(string); ok && httpEndpoint != "" {
		configs = append(configs, EndpointConfig{http, httpEndpoint, certFile, keyFile, nil})
	}

	if len(configs) == 0 {
		configs = []EndpointConfig{
			{grpc, grpcDefault, certFile, keyFile, nil},
			{http, httpDefault, certFile, keyFile, nil},
		}
	}

	return configs
}
