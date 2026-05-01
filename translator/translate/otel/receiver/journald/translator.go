// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package journald

import (
	"fmt"

	journaldinput "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/journald"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/filestorage"
)

type translator struct {
	name    string
	factory receiver.Factory
}

type translatorWithConfig struct {
	*translator
	units    []string
	priority string
	matches  []journaldinput.MatchConfig
}

var _ common.ComponentTranslator = (*translator)(nil)
var _ common.ComponentTranslator = (*translatorWithConfig)(nil)

func NewTranslator() common.ComponentTranslator {
	return NewTranslatorWithName("")
}

func NewTranslatorWithName(name string) common.ComponentTranslator {
	return &translator{
		name:    name,
		factory: journaldreceiver.NewFactory(),
	}
}

func NewTranslatorWithConfig(name string, units []string, priority string, matches []journaldinput.MatchConfig) common.ComponentTranslator {
	return &translatorWithConfig{
		translator: &translator{
			name:    name,
			factory: journaldreceiver.NewFactory(),
		},
		units:    units,
		priority: priority,
		matches:  matches,
	}
}

func NewTranslatorWithUnits(name string, units []string) common.ComponentTranslator {
	return NewTranslatorWithConfig(name, units, "", nil)
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	journaldKey := common.ConfigKey(common.LogsKey, "logs_collected", "journald")
	if conf == nil || !conf.IsSet(journaldKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: journaldKey}
	}

	cfg := t.factory.CreateDefaultConfig().(*journaldreceiver.JournaldConfig)

	// Get the journald configuration from logs.logs_collected.journald
	journaldConf, err := conf.Sub(journaldKey)
	if err != nil {
		return nil, fmt.Errorf("error getting journald configuration: %w", err)
	}
	if journaldConf == nil {
		return nil, fmt.Errorf("journald configuration not found")
	}

	collectList := journaldConf.Get("collect_list")
	if collectList == nil {
		return nil, fmt.Errorf("collect_list not found in journald configuration")
	}

	// For now, we'll use the first collect_list entry to configure the receiver
	// In a full implementation, we'd need to create multiple receivers or handle multiple configs
	collectListSlice, ok := collectList.([]interface{})
	if !ok || len(collectListSlice) == 0 {
		return nil, fmt.Errorf("collect_list is empty or invalid")
	}

	firstConfig, ok := collectListSlice[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid collect_list entry")
	}

	// Configure units if specified
	if units, ok := firstConfig["units"].([]interface{}); ok {
		cfg.InputConfig.Units = make([]string, len(units))
		for i, unit := range units {
			if unitStr, ok := unit.(string); ok {
				cfg.InputConfig.Units[i] = unitStr
			}
		}
	}

	// Configure priority if specified, default to info
	cfg.InputConfig.Priority = "info"
	if priority, ok := firstConfig["priority"].(string); ok && priority != "" {
		cfg.InputConfig.Priority = priority
	}

	// Configure matches if specified
	if matches, ok := firstConfig["matches"].([]interface{}); ok {
		for _, match := range matches {
			if matchMap, ok := match.(map[string]interface{}); ok {
				mc := make(journaldinput.MatchConfig)
				for k, v := range matchMap {
					if vs, ok := v.(string); ok {
						mc[k] = vs
					}
				}
				if len(mc) > 0 {
					cfg.InputConfig.Matches = append(cfg.InputConfig.Matches, mc)
				}
			}
		}
	}

	// Set storage for cursor persistence
	storageID := filestorage.StorageID
	cfg.BaseConfig.StorageID = &storageID

	return cfg, nil
}

func (t *translatorWithConfig) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*journaldreceiver.JournaldConfig)

	// Configure units from the provided units slice
	if len(t.units) > 0 {
		cfg.InputConfig.Units = make([]string, len(t.units))
		copy(cfg.InputConfig.Units, t.units)
	}

	// Configure priority, default to info
	if t.priority != "" {
		cfg.InputConfig.Priority = t.priority
	} else {
		cfg.InputConfig.Priority = "info"
	}

	// Configure matches
	if len(t.matches) > 0 {
		cfg.InputConfig.Matches = make([]journaldinput.MatchConfig, len(t.matches))
		for i, m := range t.matches {
			cfg.InputConfig.Matches[i] = m
		}
	}

	// Set storage for cursor persistence
	storageID := filestorage.StorageID
	cfg.BaseConfig.StorageID = &storageID

	return cfg, nil
}