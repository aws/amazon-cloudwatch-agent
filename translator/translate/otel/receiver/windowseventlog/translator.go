// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package windowseventlog

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/filestorage"
)

type translator struct {
	name     string
	channel  string
	raw      bool
	resource map[string]string
	factory  receiver.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator(name, channel string, raw bool, resource map[string]string) common.ComponentTranslator {
	return &translator{
		name:     name,
		channel:  channel,
		raw:      raw,
		resource: resource,
		factory:  windowseventlogreceiver.NewFactory(),
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*windowseventlogreceiver.WindowsLogConfig)
	cfg.InputConfig.Channel = t.channel
	cfg.InputConfig.Raw = t.raw
	cfg.InputConfig.StartAt = "end"
	storageID := filestorage.ComponentID(common.WindowsEventsKey)
	cfg.StorageID = &storageID
	if len(t.resource) > 0 {
		cfg.InputConfig.Resource = make(map[string]helper.ExprStringConfig, len(t.resource))
		for k, v := range t.resource {
			cfg.InputConfig.Resource[k] = helper.ExprStringConfig(v)
		}
	}
	return cfg, nil
}
