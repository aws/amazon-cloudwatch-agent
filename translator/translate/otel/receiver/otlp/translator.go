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
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	defaultGrpcEndpoint           = "127.0.0.1:4317"
	defaultHttpEndpoint           = "127.0.0.1:4318"
	defaultAppSignalsGrpcEndpoint = "0.0.0.0:4315"
	defaultAppSignalsHttpEndpoint = "0.0.0.0:4316"
)

var (
	configKeys = map[component.DataType]string{
		component.DataTypeTraces:  common.ConfigKey(common.TracesKey, common.TracesCollectedKey),
		component.DataTypeMetrics: common.ConfigKey(common.LogsKey, common.MetricsCollectedKey),
	}
)

type translator struct {
	name        string
	dataType    component.DataType
	instanceNum int
	factory     receiver.Factory
}

type Option interface {
	apply(t *translator)
}

type optionFunc func(t *translator)

func (o optionFunc) apply(t *translator) {
	o(t)
}

// WithDataType determines where the translator should look to find
// the configuration.
func WithDataType(dataType component.DataType) Option {
	return optionFunc(func(t *translator) {
		t.dataType = dataType
	})
}
func WithInstanceNum(instanceNum int) Option {
	return optionFunc(func(t *translator) {
		t.instanceNum = instanceNum
	})
}

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslator(opts ...Option) common.Translator[component.Config] {
	return NewTranslatorWithName("", opts...)
}

func NewTranslatorWithName(name string, opts ...Option) common.Translator[component.Config] {
	t := &translator{name: name, instanceNum: -1, factory: otlpreceiver.NewFactory()}
	for _, opt := range opts {
		opt.apply(t)
	}
	if name == "" && t.dataType.String() != "" {
		t.name = t.dataType.String()
		if t.instanceNum != -1 {
			t.name += strconv.Itoa(t.instanceNum)
		}
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*otlpreceiver.Config)
	// init default configuration
	configBase, ok := configKeys[t.dataType]
	if !ok {
		return nil, fmt.Errorf("no config key defined for data type: %s", t.dataType)
	}
	configKey := common.ConfigKey(configBase, common.OtlpKey)
	cfg.GRPC.NetAddr.Endpoint = defaultGrpcEndpoint
	cfg.HTTP.Endpoint = defaultHttpEndpoint
	if t.name == common.AppSignals {
		configKey = common.ConfigKey(configKeys[t.dataType], common.AppSignals)
		if conf == nil || !conf.IsSet(configKey) {
			configKey = common.ConfigKey(configBase, common.AppSignalsFallback)
		}
		cfg.GRPC.NetAddr.Endpoint = defaultAppSignalsGrpcEndpoint
		cfg.HTTP.Endpoint = defaultAppSignalsHttpEndpoint
	}
	if conf == nil || !conf.IsSet(configKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: configKey}
	}

	var otlpKeyMap map[string]interface{}
	if otlpSlice := common.GetArray[any](conf, configKey); t.instanceNum != -1 && len(otlpSlice) > t.instanceNum {
		otlpKeyMap = otlpSlice[t.instanceNum].(map[string]interface{})
	} else {
		otlpKeyMap = conf.Get(configKey).(map[string]interface{})
	}
	var tlsSettings *configtls.ServerConfig
	if tls, ok := otlpKeyMap["tls"].(map[string]interface{}); ok {
		tlsSettings = &configtls.ServerConfig{}
		tlsSettings.CertFile = tls["cert_file"].(string)
		tlsSettings.KeyFile = tls["key_file"].(string)
	}
	cfg.GRPC.TLSSetting = tlsSettings
	cfg.HTTP.TLSSetting = tlsSettings

	grpcEndpoint, grpcOk := otlpKeyMap["grpc_endpoint"]
	httpEndpoint, httpOk := otlpKeyMap["http_endpoint"]

	if grpcOk {
		cfg.GRPC.NetAddr.Endpoint = grpcEndpoint.(string)
	}
	if httpOk {
		cfg.HTTP.Endpoint = httpEndpoint.(string)
	}
	return cfg, nil
}
