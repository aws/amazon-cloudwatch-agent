// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package tcp_logs

import (
	"errors"
	"fmt"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcplogreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type translator struct {
	name    string
	factory receiver.Factory
}

var _ common.Translator[component.Config] = (*translator)(nil)

var (
	baseKey           = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.Emf)
	serviceAddressKey = common.ConfigKey(baseKey, common.ServiceAddress)
)

const (
	addressSplit        = ":"
	telegrafDoubleSlash = "//"
)

// NewTranslator creates a new tcp logs receiver translator.
func NewTranslator() common.Translator[component.Config] {
	return NewTranslatorWithName("")
}

// NewTranslatorWithName creates a new tcp logs receiver translator.
func NewTranslatorWithName(name string) common.Translator[component.Config] {
	return &translator{name, tcplogreceiver.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates a tcp logs receiver config if either emf has no service address or is tcp service address
// Port can be any number that allows tcp traffic
// Address can be any valid address ex localhost 0.0.0.0 127.0.0.1
// Otel does not accept address that start with // parsing is required
// Otel address is expected as host:port
// CWA expects address as tcp:host:port or tcp://host:port
// Expected service address input is
// Not Given
// tcp://:25888
// tcp://127.0.0.1:25888
// tcp:0.0.0.0:25888
// tcp:localhost:25888
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if !conf.IsSet(baseKey) ||
		(conf.IsSet(common.ConfigKey(serviceAddressKey)) && !strings.Contains(fmt.Sprintf("%v", conf.Get(serviceAddressKey)), common.Tcp)) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: fmt.Sprintf("missing %s or tcp service address", baseKey)}
	}
	cfg := t.factory.CreateDefaultConfig().(*tcplogreceiver.TCPLogConfig)
	if !conf.IsSet(common.ConfigKey(serviceAddressKey)) {
		cfg.InputConfig.BaseConfig.ListenAddress = "0.0.0.0:25888"
	} else {
		serviceAddress := fmt.Sprintf("%v", conf.Get(serviceAddressKey))
		serviceSplit := strings.Split(serviceAddress, addressSplit)
		if len(serviceSplit) != 3 {
			return nil, errors.New("invalid service split")
		} else if serviceSplit[1] == telegrafDoubleSlash {
			serviceSplit[1] = strings.Replace(serviceSplit[1], telegrafDoubleSlash, "0.0.0.0", 1)
		} else if strings.Contains(serviceAddress, telegrafDoubleSlash) {
			serviceSplit[1] = strings.Replace(serviceSplit[1], telegrafDoubleSlash, "", 1)
		}
		cfg.InputConfig.BaseConfig.ListenAddress = serviceSplit[1] + addressSplit + serviceSplit[2]
	}
	return cfg, nil
}
