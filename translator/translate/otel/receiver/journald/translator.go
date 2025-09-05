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

type translatorWithUnits struct {
	*translator
	units []string
}

var _ common.ComponentTranslator = (*translator)(nil)
var _ common.ComponentTranslator = (*translatorWithUnits)(nil)

func NewTranslator() common.ComponentTranslator {
	return NewTranslatorWithName("")
}

func NewTranslatorWithName(name string) common.ComponentTranslator {
	return &translator{
		name:    name,
		factory: journaldreceiver.NewFactory(),
	}
}

func NewTranslatorWithUnits(name string, units []string) common.ComponentTranslator {
	return &translatorWithUnits{
		translator: &translator{
			name:    name,
			factory: journaldreceiver.NewFactory(),
		},
		units: units,
	}
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

	// Set default priority to info
	cfg.InputConfig.Priority = "info"

	// Note: Storage for cursor persistence is optional
	// We can add this later if needed for production use

	return cfg, nil
}

func (t *translatorWithUnits) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*journaldreceiver.JournaldConfig)
	
	// Configure units from the provided units slice
	if len(t.units) > 0 {
		cfg.InputConfig.Units = make([]string, len(t.units))
		copy(cfg.InputConfig.Units, t.units)
	}

	// Set default priority to info
	cfg.InputConfig.Priority = "info"

	// Note: Storage for cursor persistence is optional
	// We can add this later if needed for production use

	return cfg, nil
}