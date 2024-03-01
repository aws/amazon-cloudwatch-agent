// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package adapter

import (
	"fmt"
	"log"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	translatorconfig "github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/logs_collected/files"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/logs_collected/windows_events"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect"
	collectd "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/collectd"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/customizedmetrics"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/gpu"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/procstat"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/statsd"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/otlp"
)

const (
	defaultMetricsCollectionInterval = time.Minute
)

var (
	logKey       = common.ConfigKey(common.LogsKey, common.LogsCollectedKey)
	logMetricKey = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey)
	metricKey    = common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey)
	skipInputSet = collections.NewSet[string](files.SectionKey, windows_events.SectionKey)
)

var (
	multipleInputSet = collections.NewSet[string](
		procstat.SectionKey,
	)
	// Order by PidFile, ExeKey, Pattern Key according to the public documents
	// if multiple configuration is specified
	// https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Agent-procstat-process-metrics.html#CloudWatch-Agent-procstat-configuration
	procstatMonitoredSet = []string{
		procstat.PidFileKey,
		procstat.ExeKey,
		procstat.PatternKey,
	}
	// windowsInputSet contains all the supported metric input plugins. All others are considered custom metrics.
	// An exception would be procstat metrics
	windowsInputSet = collections.NewSet[string](
		gpu.SectionKey,
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

	// OtelReceivers is used for receivers that need to be in the same pipeline that
	// exports to Cloudwatch while not having to follow the adapter rules
	OtelReceivers = map[string]common.Translator[component.Config]{
		common.OtlpKey: otlp.NewTranslator(otlp.WithDataType(component.DataTypeMetrics)),
	}
)

// FindReceiversInConfig looks in the metrics and logs sections to determine which
// plugins require adapter translators. Logs is processed first, so any
// colliding metrics translators will override them. This follows the rule
// setup.
func FindReceiversInConfig(conf *confmap.Conf, os string) (common.TranslatorMap[component.Config], error) {
	translators := common.NewTranslatorMap[component.Config]()
	translators.Merge(fromLogs(conf))
	metricTranslators, err := fromMetrics(conf, os)
	translators.Merge(metricTranslators)
	return translators, err
}

// fromMetrics creates adapter receiver translators based on the os-specific
// metrics section in the config.
func fromMetrics(conf *confmap.Conf, os string) (common.TranslatorMap[component.Config], error) {
	translators := common.NewTranslatorMap[component.Config]()
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
func fromLinuxMetrics(conf *confmap.Conf) common.TranslatorMap[component.Config] {
	var validInputs map[string]bool
	if _, ok := conf.Get(common.ConfigKey(metricKey)).(map[string]interface{}); ok {
		rule := &metrics_collect.CollectMetrics{}
		rule.ApplyRule(conf.Get(common.ConfigKey(common.MetricsKey)))
		validInputs = rule.GetRegisteredMetrics()
	}
	return fromInputs(conf, validInputs, metricKey)
}

// fromWindowsMetrics creates a translator for each allow listed subsection
// within the metrics::metrics_collected section. See windowsInputSet for
// allow list. If non-allow-listed subsections exist, they will be grouped
// under a windows performance counter adapter translator.
func fromWindowsMetrics(conf *confmap.Conf) common.TranslatorMap[component.Config] {
	translators := common.NewTranslatorMap[component.Config]()
	if inputs, ok := conf.Get(metricKey).(map[string]interface{}); ok {
		for inputName := range inputs {
			if _, ok := OtelReceivers[inputName]; ok {
				continue
			}
			if windowsInputSet.Contains(inputName) {
				cfgKey := common.ConfigKey(metricKey, inputName)
				translators.Set(NewTranslator(toAlias(inputName), cfgKey, collections.GetOrDefault(
					defaultCollectionIntervalMap,
					inputName,
					defaultMetricsCollectionInterval,
				)))
			} else {
				translators.Merge(fromMultipleInput(conf, inputName, translatorconfig.OS_TYPE_WINDOWS))
			}
		}
	}
	return translators
}

// fromLogs creates a translator for each subsection within logs::logs_collected
// along with a socket listener translator if "emf" or "structuredlog" are present
// within the logs:metrics_collected section.
func fromLogs(conf *confmap.Conf) common.TranslatorMap[component.Config] {
	return fromInputs(conf, nil, logKey)
}

// fromInputs converts all the keys in the section into adapter translators.
func fromInputs(conf *confmap.Conf, validInputs map[string]bool, baseKey string) common.TranslatorMap[component.Config] {
	translators := common.NewTranslatorMap[component.Config]()
	if inputs, ok := conf.Get(baseKey).(map[string]interface{}); ok {
		for inputName := range inputs {
			if validInputs != nil {
				if _, ok := validInputs[inputName]; !ok {
					log.Printf("W! Ignoring unrecognized input %s", inputName)
					continue
				}
			}
			cfgKey := common.ConfigKey(baseKey, inputName)
			if skipInputSet.Contains(inputName) {
				// logs agent is separate from otel agent
				continue
			} else if measurement := common.GetArray[any](conf, common.ConfigKey(cfgKey, common.MeasurementKey)); measurement != nil && len(measurement) == 0 {
				log.Printf("W! Agent will not emit any metrics for %s due to empty measurement field ", inputName)
				continue
			} else if multipleInputSet.Contains(inputName) {
				translators.Merge(fromMultipleInput(conf, inputName, ""))
			} else {
				translators.Set(NewTranslator(toAlias(inputName), cfgKey, collections.GetOrDefault(
					defaultCollectionIntervalMap,
					inputName,
					defaultMetricsCollectionInterval,
				)))
			}
		}
	}
	return translators
}

// fromMultipleInput generates multiple receivers with unique ID depends on the number of inputs.
// Since there plugins from Telegraf that allows multiple inputs such as procstat, window_perf_counter;
// therefore, generate a hash of the monitored process (e.g exe: hash(amazon-cloudwatch-agent))
// to provide a unique identifier for the receivers and easy in compare with the alias
// https://github.com/influxdata/telegraf/blob/d8db3ca3a293bc24a9120b590984b09e2de1851a/models/running_input.go#L60
// and generate the appropriate running input when starting adapter
func fromMultipleInput(conf *confmap.Conf, inputName, os string) common.TranslatorMap[component.Config] {
	translators := common.NewTranslatorMap[component.Config]()
	cfgKey := common.ConfigKey(metricKey, inputName)

	if inputName == procstat.SectionKey {
		/*
			 For procstat metrics, telegraf allows and generates more than 2 inputs.
			[[inputs.procstat]]
				pattern = "ssm-agent"
				interval = "1s"
				fieldpass = ["memory_stack"]
				pid_finder = "native"
			[[inputs.procstat]]
				exe = "amazon-cloudwatch-agent"
				interval = "1s"
				fieldpass = ["cpu_time_system"]
				pid_finder = "native"
		*/
		for _, procStatKey := range common.GetArray[any](conf, cfgKey) {
			// Each of the procstat monitored process has their own process; therefore, overriding the interval key chain
			// and setting dirrectly
			psKey := procStatKey.(map[string]interface{})
			psCollectionInterval, _ := common.ParseDuration(psKey[common.MetricsCollectionIntervalKey])

			// Array type validation needs to be specific https://stackoverflow.com/a/47989212
			for _, procstatMonitored := range procstatMonitoredSet {
				if componentPsValue, ok := psKey[procstatMonitored]; ok {
					translators.Set(NewTranslatorWithName(
						componentPsValue.(string),
						procstat.SectionKey,
						cfgKey,
						psCollectionInterval,
						defaultMetricsCollectionInterval))
					break
				}
			}
		}
	} else if os == translatorconfig.OS_TYPE_WINDOWS && !windowsInputSet.Contains(inputName) {
		/* For customized metrics from Windows and  window performance counters metrics
		   	[[inputs.win_perf_counters.object]]
		   		ObjectName = "Processor"
		   		Instances = ["*"]
		   		Counters = ["% Idle Time", "% Interrupt Time", "% Privileged Time", "% User Time", "% Processor Time"]
		   		Measurement = "win_cpu"

		     	[[inputs.win_perf_counters.object]]
		   		ObjectName = "LogicalDisk"
		   		Instances = ["*"]
		   		Counters = ["% Idle Time", "% Disk Time","% Disk Read Time", "% Disk Write Time", "% User Time", "Current Disk Queue Length"]
		   		Measurement = "win_disk"
		*/
		translators.Set(NewTranslatorWithName(
			inputName,
			customizedmetrics.WinPerfCountersKey,
			cfgKey,
			time.Duration(0),
			defaultMetricsCollectionInterval,
		))
	}
	return translators
}

// toAlias gets the alias for the input name if it has one.
func toAlias(inputName string) string {
	return collections.GetOrDefault(aliasMap, inputName, inputName)
}
