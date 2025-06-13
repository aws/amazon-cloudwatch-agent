// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlp

import (
	_ "embed"
	"fmt"
	"strconv"

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
