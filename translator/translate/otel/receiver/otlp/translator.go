// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlp

import (
	_ "embed"
	"errors"
	"fmt"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"reflect"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/hash"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	defaultGrpcEndpoint           = "127.0.0.1:4317"
	defaultHttpEndpoint           = "127.0.0.1:4318"
	defaultAppSignalsGrpcEndpoint = "0.0.0.0:4315"
	defaultAppSignalsHttpEndpoint = "0.0.0.0:4316"
	defaultJMXHttpEndpoint        = "0.0.0.0:4314"
)

var (
	httpCache = make(map[string]*otlpreceiver.Config)
	grpcCache = make(map[string]*otlpreceiver.Config)
)

type translator struct {
	common.NameProvider
	common.IndexProvider
	configKey string
	signal    pipeline.Signal
	factory   receiver.Factory
	cfg       *otlpreceiver.Config
	err       error
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

func NewTranslator(conf *confmap.Conf, opts ...common.TranslatorOption) common.ComponentTranslator {
	t := &translator{factory: otlpreceiver.NewFactory()}
	t.SetIndex(-1)
	for _, opt := range opts {
		opt(t)
	}
	if t.Name() == "" {
		t.SetName("otlp")
	}

	var name string
	errs := make([]error, 0)
	cfg, err := t.translate(conf)

	if err != nil {
		errs = append(errs, err)
	} else {

		if cfg.HTTP != nil {
			httpEndpoint := cfg.HTTP.ServerConfig.Endpoint
			if httpCache[httpEndpoint] != nil {
				if !reflect.DeepEqual(cfg, httpCache[httpEndpoint]) {
					errs = append(errs, fmt.Errorf("same endpoint used in different otlp receivers: %s", httpEndpoint))
				}
			} else {
				httpCache[httpEndpoint] = cfg
				name += httpEndpoint
			}
		}

		if cfg.GRPC != nil {
			grpcEndpoint := cfg.GRPC.NetAddr.Endpoint
			if grpcCache[grpcEndpoint] != nil {
				if !reflect.DeepEqual(cfg, grpcCache[grpcEndpoint]) {
					errs = append(errs, fmt.Errorf("same endpoint used in different otlp receivers: %s", grpcEndpoint))
				}
			} else {
				grpcCache[grpcEndpoint] = cfg
				name += grpcEndpoint
			}
		}

		t.SetName(t.Name() + "/" + hash.HashName(name))
	}

	if len(errs) != 0 {
		t.cfg = nil
		t.err = errors.Join(errs...)
	} else {
		t.cfg = cfg
		t.err = nil
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.Name())
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	return t.cfg, t.err
}

func (t *translator) translate(conf *confmap.Conf) (*otlpreceiver.Config, error) {
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
