// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package adapter

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/internal/util/collections"
	translatorconfig "github.com/aws/private-amazon-cloudwatch-agent-staging/translator/config"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/logs/logs_collected/files"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/logs/logs_collected/windows_events"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/logs/metrics_collected/emf"
	collectd "github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/metrics/metrics_collect/collectd"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/metrics/metrics_collect/customizedmetrics"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/metrics/metrics_collect/gpu"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/metrics/metrics_collect/procstat"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/metrics/metrics_collect/statsd"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

const (
	defaultMetricsCollectionInterval = time.Minute
)

var (
	// windowsInputSet contains all the supported metric input plugins.
	// All others are considered custom metrics.
	windowsInputSet = collections.NewSet(
		gpu.SectionKey,
		procstat.SectionKey,
		statsd.SectionKey,
	)
	// aliasMap contains mappings for all input plugins that use another
	// name in Telegraf.
	aliasMap = map[string]string{
		collectd.SectionKey:       collectd.SectionMappedKey,
		files.SectionKey:          files.SectionMappedKey,
		gpu.SectionKey:            gpu.SectionMappedKey,
		windows_events.SectionKey: windows_events.SectionMappedKey,
	}
	// defaultCollectionIntervalMap contains all input plugins that have a
	// different default interval.
	defaultCollectionIntervalMap = map[string]time.Duration{
		statsd.SectionKey: 10 * time.Second,
	}
)

// FindReceiversInConfig looks in the metrics and logs sections to determine which
// plugins require adapter translators. Logs is processed first, so any
// colliding metrics translators will override them. This follows the rule
// setup.
func FindReceiversInConfig(conf *confmap.Conf, os string) (common.TranslatorMap[config.Receiver], error) {
	translators := common.NewTranslatorMap[config.Receiver]()
	translators.Merge(fromLogs(conf))
	metricTranslators, err := fromMetrics(conf, os)
	translators.Merge(metricTranslators)
	return translators, err
}

// fromMetrics creates adapter receiver translators based on the os-specific
// metrics section in the config.
func fromMetrics(conf *confmap.Conf, os string) (common.TranslatorMap[config.Receiver], error) {
	translators := common.NewTranslatorMap[config.Receiver]()
	switch os {
	case translatorconfig.OS_TYPE_LINUX, translatorconfig.OS_TYPE_DARWIN:
		translators.Merge(fromLinuxMetrics(conf))
	case translatorconfig.OS_TYPE_WINDOWS:
		translators.Merge(fromWindowsMetrics(conf))
	default:
		return nil, fmt.Errorf("unsupported OS: %v", os)
	}
	return translators, nil
}

// fromLinuxMetrics creates a translator for each subsection within the
// metrics::metrics_collected section of the config. Can be anything.
func fromLinuxMetrics(conf *confmap.Conf) common.TranslatorMap[config.Receiver] {
	return fromInputs(conf, common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey))
}

// fromWindowsMetrics creates a translator for each allow listed subsection
// within the metrics::metrics_collected section. See windowsInputSet for
// allow list. If non-allow-listed subsections exist, they will be grouped
// under a windows performance counter adapter translator.
func fromWindowsMetrics(conf *confmap.Conf) common.TranslatorMap[config.Receiver] {
	translators := common.NewTranslatorMap[config.Receiver]()
	key := common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey)
	if inputs, ok := conf.Get(key).(map[string]interface{}); ok {
		var isCustomMetricsPresent bool
		for inputName := range inputs {
			if windowsInputSet.Contains(inputName) {
				cfgKey := common.ConfigKey(key, inputName)
				translators.Add(NewTranslator(toAlias(inputName), cfgKey, collections.GetOrDefault(
					defaultCollectionIntervalMap,
					inputName,
					defaultMetricsCollectionInterval,
				)))
			} else {
				isCustomMetricsPresent = true
			}
		}
		if isCustomMetricsPresent {
			translators.Add(NewTranslator(
				customizedmetrics.Win_Perf_Counters_Key,
				common.MetricsKey,
				defaultMetricsCollectionInterval,
			))
		}
	}
	return translators
}

// fromLogs creates a translator for each subsection within logs::logs_collected
// along with a socket listener translator if "emf" or "structuredlog" are present
// within the logs:metrics_collected section.
func fromLogs(conf *confmap.Conf) common.TranslatorMap[config.Receiver] {
	translators := common.NewTranslatorMap[config.Receiver]()
	key := common.ConfigKey(common.LogsKey, common.MetricsCollectedKey)
	for _, socketListenerKey := range []string{emf.SectionKey, emf.SectionKeyStructuredLog} {
		cfgKey := common.ConfigKey(key, socketListenerKey)
		if conf.IsSet(cfgKey) {
			translators.Add(NewTranslator(collectd.SectionMappedKey, cfgKey, defaultMetricsCollectionInterval))
			break
		}
	}
	translators.Merge(fromInputs(conf, common.ConfigKey(common.LogsKey, common.LogsCollectedKey)))
	return translators
}

// fromInputs converts all the keys in the section into adapter translators.
func fromInputs(conf *confmap.Conf, baseKey string) common.TranslatorMap[config.Receiver] {
	translators := common.NewTranslatorMap[config.Receiver]()
	if inputs, ok := conf.Get(baseKey).(map[string]interface{}); ok {
		for inputName := range inputs {
			cfgKey := common.ConfigKey(baseKey, inputName)
			translators.Add(NewTranslator(toAlias(inputName), cfgKey, collections.GetOrDefault(
				defaultCollectionIntervalMap,
				inputName,
				defaultMetricsCollectionInterval,
			)))
		}
	}
	return translators
}

// toAlias gets the alias for the input name if it has one.
func toAlias(inputName string) string {
	return collections.GetOrDefault(aliasMap, inputName, inputName)
}
