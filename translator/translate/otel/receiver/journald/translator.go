// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package journald

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/journald"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type translator struct {
	name    string
	factory component.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

const (
	directoryKey      = "directory"
	filesKey          = "files"
	unitsKey          = "units"
	identifiersKey    = "identifiers"
	priorityKey       = "priority"
	grepKey           = "grep"
	matchesKey        = "matches"
	dmesgKey          = "dmesg"
	allKey            = "all"
	startAtKey        = "start_at"
	logGroupNameKey   = "log_group_name"
	logStreamNameKey  = "log_stream_name"
)

var (
	baseKey = common.ConfigKey(common.LogsKey, common.LogsCollectedKey, common.JournaldKey)
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

// Translate creates a journald receiver config based on the provided configuration.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if !conf.IsSet(baseKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: baseKey}
	}

	cfg := t.factory.CreateDefaultConfig().(*journaldreceiver.JournaldConfig)

	// Configure directory
	if directory, ok := common.GetString(conf, common.ConfigKey(baseKey, directoryKey)); ok {
		cfg.InputConfig.Directory = &directory
	}

	// Configure files
	if files := common.GetArray[string](conf, common.ConfigKey(baseKey, filesKey)); files != nil {
		cfg.InputConfig.Files = files
	}

	// Configure units
	if units := common.GetArray[string](conf, common.ConfigKey(baseKey, unitsKey)); units != nil {
		cfg.InputConfig.Units = units
	}

	// Configure identifiers
	if identifiers := common.GetArray[string](conf, common.ConfigKey(baseKey, identifiersKey)); identifiers != nil {
		cfg.InputConfig.Identifiers = identifiers
	}

	// Configure priority
	if priority, ok := common.GetString(conf, common.ConfigKey(baseKey, priorityKey)); ok {
		cfg.InputConfig.Priority = priority
	}

	// Configure grep
	if grep, ok := common.GetString(conf, common.ConfigKey(baseKey, grepKey)); ok {
		cfg.InputConfig.Grep = grep
	}

	// Configure matches
	if matchesRaw := conf.Get(common.ConfigKey(baseKey, matchesKey)); matchesRaw != nil {
		if matchesArray, ok := matchesRaw.([]interface{}); ok {
			var matches []journald.MatchConfig
			for _, match := range matchesArray {
				if matchMap, ok := match.(map[string]interface{}); ok {
					convertedMatch := make(journald.MatchConfig)
					for k, v := range matchMap {
						if strVal, ok := v.(string); ok {
							convertedMatch[k] = strVal
						}
					}
					matches = append(matches, convertedMatch)
				}
			}
			cfg.InputConfig.Matches = matches
		}
	}

	// Configure dmesg
	if dmesg, ok := common.GetBool(conf, common.ConfigKey(baseKey, dmesgKey)); ok {
		cfg.InputConfig.Dmesg = dmesg
	}

	// Configure all
	if all, ok := common.GetBool(conf, common.ConfigKey(baseKey, allKey)); ok {
		cfg.InputConfig.All = all
	}

	// Configure start_at
	if startAt, ok := common.GetString(conf, common.ConfigKey(baseKey, startAtKey)); ok {
		cfg.InputConfig.StartAt = startAt
	}

	return cfg, nil
}