// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package journald

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type translator struct {
	name    string
	factory receiver.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

var (
	baseKey = common.JournaldConfigKey
)

// NewTranslator creates a new journald receiver translator.
func NewTranslator() common.ComponentTranslator {
	return NewTranslatorWithName("")
}

// NewTranslatorWithName creates a new journald receiver translator with a specific name.
func NewTranslatorWithName(name string) common.ComponentTranslator {
	return &translator{name, journaldreceiver.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate converts CloudWatch Agent journald configuration to OpenTelemetry journald receiver configuration.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !conf.IsSet(baseKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: baseKey}
	}

	cfg := t.factory.CreateDefaultConfig().(*journaldreceiver.JournaldConfig)
	if _, ok := conf.Get(baseKey).(map[string]interface{}); !ok {
		return nil, fmt.Errorf("journald configuration must be an object")
	}


	if units := common.GetArray[string](conf, common.ConfigKey(baseKey, "units")); units != nil {
		cfg.InputConfig.Units = units
	}

	if priority, ok := common.GetString(conf, common.ConfigKey(baseKey, "priority")); ok {
		cfg.InputConfig.Priority = priority
	}

	if directory, ok := common.GetString(conf, common.ConfigKey(baseKey, "directory")); ok {
		cfg.InputConfig.Directory = &directory
	}

	if files := common.GetArray[string](conf, common.ConfigKey(baseKey, "files")); files != nil {
		cfg.InputConfig.Files = files
	}

	if identifiers := common.GetArray[string](conf, common.ConfigKey(baseKey, "identifiers")); identifiers != nil {
		cfg.InputConfig.Identifiers = identifiers
	}

	if grep, ok := common.GetString(conf, common.ConfigKey(baseKey, "grep")); ok {
		cfg.InputConfig.Grep = grep
	}

	if dmesg, ok := common.GetBool(conf, common.ConfigKey(baseKey, "dmesg")); ok {
		cfg.InputConfig.Dmesg = dmesg
	}

	if all, ok := common.GetBool(conf, common.ConfigKey(baseKey, "all")); ok {
		cfg.InputConfig.All = all
	}

	if namespace, ok := common.GetString(conf, common.ConfigKey(baseKey, "namespace")); ok {
		cfg.InputConfig.Namespace = namespace
	}

	// Set start_at based on "since" parameter
	if since, ok := common.GetString(conf, common.ConfigKey(baseKey, "since")); ok && since != "" {
		cfg.InputConfig.StartAt = "beginning"
	} else {
		cfg.InputConfig.StartAt = "end"
	}

	return cfg, nil
}