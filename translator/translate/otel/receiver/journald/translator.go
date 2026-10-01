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
	name     string
	factory  receiver.Factory
	units    []string
	priority string
	matches  []journaldinput.MatchConfig
	mode     string
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslatorWithConfig(name string, units []string, priority string, matches []journaldinput.MatchConfig, mode string) common.ComponentTranslator {
	return &translator{
		name:     name,
		factory:  journaldreceiver.NewFactory(),
		units:    units,
		priority: priority,
		matches:  matches,
		mode:     mode,
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

	// Map the optional agent-config "mode" onto the receiver's backend
	// selector. An empty/absent mode is left untouched so the contrib
	// default (ModeJournalctl) is preserved exactly. An unrecognized value
	// is a hard error rather than a silent fallback: falling back to
	// journalctl inside the agent's FROM-scratch container image (which has
	// no journalctl binary) is precisely the failure this option exists to
	// prevent.
	switch t.mode {
	case "":
		// preserve the contrib default
	case journaldreceiver.ModeNative:
		cfg.Mode = journaldreceiver.ModeNative
	case journaldreceiver.ModeJournalctl:
		cfg.Mode = journaldreceiver.ModeJournalctl
	default:
		return nil, fmt.Errorf("invalid journald mode %q (must be %q or %q)", t.mode, journaldreceiver.ModeJournalctl, journaldreceiver.ModeNative)
	}

	storageID := filestorage.ComponentID()
	cfg.StorageID = &storageID

	return cfg, nil
}
