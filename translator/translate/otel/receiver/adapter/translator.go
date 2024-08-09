// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package adapter

import (
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/hash"
	"github.com/aws/amazon-cloudwatch-agent/receiver/adapter"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type translator struct {
	name string
	// cfgType determines the type set in the config.
	cfgType component.Type
	// cfgKey represents the flattened path to the section in the
	// JSON config that must be present for the translator to work.
	// See otel.ConfigKey.
	cfgKey string

	// preferMetricCollectionInterval is an option to using the preferaable metric collection interval before
	// using the interval key chain and defaultMetricCollectionInterval
	preferMetricCollectionInterval time.Duration

	// defaultMetricCollectionInterval is the fallback interval if it
	// it is not present in the interval keychain.
	defaultMetricCollectionInterval time.Duration
}

var _ common.Translator[component.Config] = (*translator)(nil)

// NewTranslator creates a new adapter receiver translator.
func NewTranslator(inputName, cfgKey string, defaultMetricCollectionInterval time.Duration) common.Translator[component.Config] {
	return NewTranslatorWithName("", inputName, cfgKey, time.Duration(0), defaultMetricCollectionInterval)
}

func NewTranslatorWithName(name, inputName, cfgKey string, preferMetricCollectionInterval, defaultMetricCollectionInterval time.Duration) common.Translator[component.Config] {
	return &translator{name, adapter.Type(inputName), cfgKey, preferMetricCollectionInterval, defaultMetricCollectionInterval}
}

func (t *translator) ID() component.ID {
	// There are two telegraf input:
	// * Single input which create single receiver (e.g cpu, mem)
	// * Single input which create multiple receivers (e.g procstat, wind_perf_counters)
	// For the former one, we will create an empty hash while the later one
	// will be hash (e.g procstat can monitor pid file which will create
	// a complexity receiver name if using non-hash)
	return component.NewIDWithName(t.cfgType, hash.HashName(t.name))
}

// Translate creates an adapter receiver config if the section set on
// the translator exists. Tries to get the collection interval from
// the section key. Falls back on the agent section if it is not present.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !conf.IsSet(t.cfgKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: t.cfgKey}
	}
	cfg := &adapter.Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		AliasName:        t.ID().String(),
	}

	intervalKeyChain := []string{
		common.ConfigKey(t.cfgKey, common.MetricsCollectionIntervalKey),
		common.ConfigKey(common.AgentKey, common.MetricsCollectionIntervalKey),
	}

	cfg.AliasName = t.name
	// The fall back interval is 0 when there is no plugin's collection interval or the plugin's collection interval cannot  be scraped.
	// Therefore, using 0 as a gate for procstat plugin
	if t.preferMetricCollectionInterval != time.Duration(0) {
		cfg.CollectionInterval = t.preferMetricCollectionInterval
	} else {
		cfg.CollectionInterval = common.GetOrDefaultDuration(conf, intervalKeyChain, t.defaultMetricCollectionInterval)
	}

	return cfg, nil
}
