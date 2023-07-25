// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlp

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	defaultGrpcEndpoint = "127.0.0.1:4317"
	defaultHttpEndpoint = "127.0.0.1:4318"
)

var (
	configKeys = map[component.DataType]string{
		component.DataTypeTraces: common.ConfigKey(common.TracesKey, common.TracesCollectedKey, common.OtlpKey),
	}
)

type translator struct {
	name     string
	dataType component.DataType
	factory  receiver.Factory
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

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslator(opts ...Option) common.Translator[component.Config] {
	t := &translator{factory: otlpreceiver.NewFactory()}
	for _, opt := range opts {
		opt.apply(t)
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	configKey, ok := configKeys[t.dataType]
	if !ok {
		return nil, fmt.Errorf("no config key defined for data type: %s", t.dataType)
	}
	if conf == nil || !conf.IsSet(configKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: configKey}
	}
	cfg := t.factory.CreateDefaultConfig().(*otlpreceiver.Config)
	cfg.GRPC.NetAddr.Endpoint = defaultGrpcEndpoint
	cfg.HTTP.Endpoint = defaultHttpEndpoint
	if endpoint, ok := common.GetString(conf, common.ConfigKey(configKey, "grpc_endpoint")); ok {
		cfg.GRPC.NetAddr.Endpoint = endpoint
	}
	if endpoint, ok := common.GetString(conf, common.ConfigKey(configKey, "http_endpoint")); ok {
		cfg.HTTP.Endpoint = endpoint
	}
	return cfg, nil
}
