// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package journald

import (
	journaldinput "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/journald"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/filestorage"
)

type translator struct {
	name     string
	factory  receiver.Factory
	units    []string
	priority string
	matches  []journaldinput.MatchConfig
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslatorWithConfig(name string, units []string, priority string, matches []journaldinput.MatchConfig) common.ComponentTranslator {
	return &translator{
		name:     name,
		factory:  journaldreceiver.NewFactory(),
		units:    units,
		priority: priority,
		matches:  matches,
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*journaldreceiver.JournaldConfig)

	if len(t.units) > 0 {
		cfg.InputConfig.Units = make([]string, len(t.units))
		copy(cfg.InputConfig.Units, t.units)
	}

	if t.priority != "" {
		cfg.InputConfig.Priority = t.priority
	} else {
		cfg.InputConfig.Priority = "info"
	}

	if len(t.matches) > 0 {
		cfg.InputConfig.Matches = make([]journaldinput.MatchConfig, len(t.matches))
		copy(cfg.InputConfig.Matches, t.matches)
	}

	storageID := filestorage.StorageComponentID()
	cfg.StorageID = &storageID

	return cfg, nil
}
