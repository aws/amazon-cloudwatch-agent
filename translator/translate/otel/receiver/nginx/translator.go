// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nginx

import (
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nginxreceiver"
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
	baseKey = common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, "nginx")
)

// NewTranslator creates a new udp logs receiver translator.
func NewTranslator() common.Translator[component.Config] {
	return NewTranslatorWithName("")
}

// NewTranslatorWithName creates a new udp logs receiver translator.
func NewTranslatorWithName(name string) common.Translator[component.Config] {
	return &translator{name, nginxreceiver.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if !conf.IsSet(baseKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: fmt.Sprintf("missing %s for nginx", baseKey)}
	}
	cfg := t.factory.CreateDefaultConfig().(*nginxreceiver.Config)
	return cfg, nil
}
