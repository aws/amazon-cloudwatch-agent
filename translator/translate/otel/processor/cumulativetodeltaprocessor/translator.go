// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cumulativetodeltaprocessor

import (
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	// Match types are in internal package from contrib
	// Strict is the FilterType for filtering by exact string matches.
	strict = "strict"
)

var (
	netKey    = common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.NetKey)
	diskioKey = common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.DiskIOKey)

	exclusions = map[string][]string{
		// DiskIO and Net Metrics are cumulative metrics
		// DiskIO: https://github.com/shirou/gopsutil/blob/master/disk/disk.go#L32-L47
		// Net: https://github.com/shirou/gopsutil/blob/master/net/net.go#L13-L25
		// https://github.com/aws/amazon-cloudwatch-agent/blob/5ace5aa6d817684cf82f4e6aa82d9596fb56d74b/translator/translate/metrics/util/deltasutil.go#L33-L65
		diskioKey: {"iops_in_progress", "diskio_iops_in_progress"},
	}
)

func WithDiskIONetKeys() common.TranslatorOption {
	return WithConfigKeys(diskioKey, netKey)
}

func WithConfigKeys(keys ...string) common.TranslatorOption {
	return func(target any) {
		if setter, ok := target.(*translator); ok {
			setter.keys = keys
		}
	}
}

type translator struct {
	factory processor.Factory
	common.NameProvider
	keys []string
}

var _ common.Translator[component.Config] = (*translator)(nil)
var _ common.NameSetter = (*translator)(nil)

func NewTranslator(opts ...common.TranslatorOption) common.Translator[component.Config] {
	t := &translator{factory: cumulativetodeltaprocessor.NewFactory()}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.Name())
}

// Translate creates a processor config based on the fields in the
// Metrics section of the JSON config.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !common.IsAnySet(conf, t.keys) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: strings.Join(t.keys, " or ")}
	}

	cfg := t.factory.CreateDefaultConfig().(*cumulativetodeltaprocessor.Config)

	excludeMetrics := t.getExcludeMetrics(conf)
	if len(excludeMetrics) != 0 {
		cfg.Exclude.MatchType = strict
		cfg.Exclude.Metrics = excludeMetrics
	}
	return cfg, nil
}

func (t *translator) getExcludeMetrics(conf *confmap.Conf) []string {
	var excludeMetricNames []string
	for _, key := range t.keys {
		exclude, ok := exclusions[key]
		if ok && conf.IsSet(key) {
			excludeMetricNames = append(excludeMetricNames, exclude...)
		}
	}
	return excludeMetricNames
}
