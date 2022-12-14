// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package adapter

import (
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/receiver/adapter"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

type translator struct {
	// cfgType determines the type set in the config.
	cfgType component.Type
	// cfgKey represents the flattened path to the section in the
	// JSON config that must be present for the translator to work.
	// See otel.ConfigKey.
	cfgKey string
	// defaultMetricCollectionInterval is the fallback interval if it
	// it is not present in the interval keychain.
	defaultMetricCollectionInterval time.Duration
}

var _ common.Translator[component.Config] = (*translator)(nil)

// NewTranslator creates a new adapter receiver translator.
func NewTranslator(inputName string, cfgKey string, defaultMetricCollectionInterval time.Duration) common.Translator[component.Config] {
	return &translator{adapter.Type(inputName), cfgKey, defaultMetricCollectionInterval}
}

func (t *translator) Type() component.Type {
	return t.cfgType
}

// Translate creates an adapter receiver config if the section set on
// the translator exists. Tries to get the collection interval from
// the section key. Falls back on the agent section if it is not present.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !conf.IsSet(t.cfgKey) {
		return nil, &common.MissingKeyError{Type: t.Type(), JsonKey: t.cfgKey}
	}
	cfg := &adapter.Config{
		ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(t.Type()),
	}
	intervalKeyChain := []string{
		common.ConfigKey(t.cfgKey, common.MetricsCollectionIntervalKey),
		common.ConfigKey(common.AgentKey, common.MetricsCollectionIntervalKey),
	}
	cfg.CollectionInterval = common.GetOrDefaultDuration(conf, intervalKeyChain, t.defaultMetricCollectionInterval)
	return cfg, nil
}
