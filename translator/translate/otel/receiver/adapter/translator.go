// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package adapter

import (
	"time"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/receiver/adapter"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

const (
	defaultMetricsCollectionInterval = time.Minute
)

type translator struct {
	// cfgType determines the type set in the config.
	cfgType config.Type
	// cfgKey represents the flattened path to the section in the
	// JSON config that must be present for the translator to work.
	// See otel.ConfigKey.
	cfgKey string
}

var _ common.Translator[config.Receiver] = (*translator)(nil)

// NewTranslator creates a new adapter receiver translator.
func NewTranslator(inputName string, cfgKey string) common.Translator[config.Receiver] {
	return &translator{adapter.Type(inputName), cfgKey}
}

func (t *translator) Type() config.Type {
	return t.cfgType
}

// Translate creates an adapter receiver config if the section set on
// the translator exists. Tries to get the collection interval from
// the section key. Falls back on the agent section if it is not present.
func (t *translator) Translate(conf *confmap.Conf) (config.Receiver, error) {
	if conf == nil || !conf.IsSet(t.cfgKey) {
		return nil, &common.MissingKeyError{Type: t.Type(), JsonKey: t.cfgKey}
	}
	cfg := &adapter.Config{
		ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(t.Type()),
	}
	intervalKeyChain := []string{t.cfgKey, common.AgentKey}
	cfg.CollectionInterval = getCollectionInterval(conf, intervalKeyChain)
	return cfg, nil
}

// getCollectionInterval from the first section with a parsable duration.
// If none are found, uses the defaultMetricsCollectionInterval.
func getCollectionInterval(conf *confmap.Conf, intervalKeyChain []string) time.Duration {
	for _, cfgKey := range intervalKeyChain {
		key := common.ConfigKey(cfgKey, common.MetricsCollectionIntervalKey)
		duration, ok := common.GetDuration(conf, key)
		if !ok {
			continue
		}
		return duration
	}
	return defaultMetricsCollectionInterval
}
