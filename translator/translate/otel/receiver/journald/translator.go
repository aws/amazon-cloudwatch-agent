// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package journald

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

var (
	baseKey = common.ConfigKey(common.LogsKey, common.LogsCollectedKey, common.JournaldKey)
) 

const (
	startAtEnd     = "end"
	priorityDebug  = "debug"
)

type translator struct {
	name    string
	factory component.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator() common.ComponentTranslator {
	return NewTranslatorWithName("")
}

func NewTranslatorWithName(name string) common.ComponentTranslator {
	return &translator{name, journaldreceiver.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if !conf.IsSet(baseKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: baseKey}
	}

	cfg := t.factory.CreateDefaultConfig().(*journaldreceiver.JournaldConfig)
	cfg.InputConfig.Priority = priorityDebug
	cfg.InputConfig.StartAt = startAtEnd

	return cfg, nil
}
