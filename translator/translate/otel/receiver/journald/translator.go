//go:build linux

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

var baseKey = common.ConfigKey(common.LogsKey, common.LogsCollectedKey, "journald")

// NewTranslator creates a new journald receiver translator.
func NewTranslator() common.ComponentTranslator {
	return NewTranslatorWithName("")
}

// NewTranslatorWithName creates a new journald receiver translator with a name.
func NewTranslatorWithName(name string) common.ComponentTranslator {
	return &translator{name, journaldreceiver.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates a journald receiver config from CloudWatch Agent config.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if !conf.IsSet(baseKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: baseKey}
	}

	cfg := t.factory.CreateDefaultConfig().(*journaldreceiver.JournaldConfig)

	// Map units
	if unitsKey := common.ConfigKey(baseKey, "units"); conf.IsSet(unitsKey) {
		if units, ok := conf.Get(unitsKey).([]interface{}); ok {
			cfg.InputConfig.Units = make([]string, len(units))
			for i, u := range units {
				cfg.InputConfig.Units[i] = fmt.Sprintf("%v", u)
			}
		}
	}

	// Map priority
	if priorityKey := common.ConfigKey(baseKey, "priority"); conf.IsSet(priorityKey) {
		cfg.InputConfig.Priority = fmt.Sprintf("%v", conf.Get(priorityKey))
	}

	// Map start_at
	if startAtKey := common.ConfigKey(baseKey, "start_at"); conf.IsSet(startAtKey) {
		cfg.InputConfig.StartAt = fmt.Sprintf("%v", conf.Get(startAtKey))
	}

	// Map directory
	if directoryKey := common.ConfigKey(baseKey, "directory"); conf.IsSet(directoryKey) {
		dir := fmt.Sprintf("%v", conf.Get(directoryKey))
		cfg.InputConfig.Directory = &dir
	}

	// Map files
	if filesKey := common.ConfigKey(baseKey, "files"); conf.IsSet(filesKey) {
		if files, ok := conf.Get(filesKey).([]interface{}); ok {
			cfg.InputConfig.Files = make([]string, len(files))
			for i, f := range files {
				cfg.InputConfig.Files[i] = fmt.Sprintf("%v", f)
			}
		}
	}

	// Map identifiers
	if identifiersKey := common.ConfigKey(baseKey, "identifiers"); conf.IsSet(identifiersKey) {
		if identifiers, ok := conf.Get(identifiersKey).([]interface{}); ok {
			cfg.InputConfig.Identifiers = make([]string, len(identifiers))
			for i, id := range identifiers {
				cfg.InputConfig.Identifiers[i] = fmt.Sprintf("%v", id)
			}
		}
	}

	// Map grep
	if grepKey := common.ConfigKey(baseKey, "grep"); conf.IsSet(grepKey) {
		cfg.InputConfig.Grep = fmt.Sprintf("%v", conf.Get(grepKey))
	}

	// Map dmesg
	if dmesgKey := common.ConfigKey(baseKey, "dmesg"); conf.IsSet(dmesgKey) {
		if dmesg, ok := conf.Get(dmesgKey).(bool); ok {
			cfg.InputConfig.Dmesg = dmesg
		}
	}

	// Map all
	if allKey := common.ConfigKey(baseKey, "all"); conf.IsSet(allKey) {
		if all, ok := conf.Get(allKey).(bool); ok {
			cfg.InputConfig.All = all
		}
	}

	// Map namespace
	if namespaceKey := common.ConfigKey(baseKey, "namespace"); conf.IsSet(namespaceKey) {
		cfg.InputConfig.Namespace = fmt.Sprintf("%v", conf.Get(namespaceKey))
	}

	// Map matches
	if matchesKey := common.ConfigKey(baseKey, "matches"); conf.IsSet(matchesKey) {
		if matches, ok := conf.Get(matchesKey).([]interface{}); ok {
			cfg.InputConfig.Matches = make([]map[string]string, len(matches))
			for i, m := range matches {
				if matchMap, ok := m.(map[string]interface{}); ok {
					cfg.InputConfig.Matches[i] = make(map[string]string)
					for k, v := range matchMap {
						cfg.InputConfig.Matches[i][k] = fmt.Sprintf("%v", v)
					}
				}
			}
		}
	}

	return cfg, nil
}
