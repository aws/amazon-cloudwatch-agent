// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlp

import (
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

type translator struct {
	common.NameProvider
	common.IndexProvider
	signal  pipeline.Signal
	factory receiver.Factory
	cfg     component.Config
	err     error
}

var (
	configCache = make(map[common.OtlpEndpointConfig]component.Config)
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

func NewTranslator(otlpConfig common.OtlpEndpointConfig, opts ...common.TranslatorOption) common.ComponentTranslator {
	t := &translator{factory: otlpreceiver.NewFactory()}
	t.SetIndex(-1)
	for _, opt := range opts {
		opt(t)
	}

	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	// set name as "{type - http or grpc}" then appends "_{port}" if available
	t.SetName(string(otlpConfig.Protocol))
	if parts := strings.Split(otlpConfig.Endpoint, ":"); len(parts) > 1 {
		t.SetName(t.Name() + "_" + parts[1])
	}
	if existingCfg, exists := configCache[otlpConfig]; exists {
		t.cfg = existingCfg
		return t
	}

	cfg := t.factory.CreateDefaultConfig().(*otlpreceiver.Config)

	tlsSettings := &configtls.ServerConfig{}
	if otlpConfig.CertFile != "" || otlpConfig.KeyFile != "" {
		tlsSettings.CertFile = otlpConfig.CertFile
		tlsSettings.KeyFile = otlpConfig.KeyFile
	}

	if otlpConfig.Protocol == common.HTTP {
		cfg.GRPC = nil
		cfg.HTTP.ServerConfig.Endpoint = otlpConfig.Endpoint
		cfg.HTTP.ServerConfig.TLSSetting = tlsSettings
	} else {
		cfg.HTTP = nil
		cfg.GRPC.NetAddr.Endpoint = otlpConfig.Endpoint
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
