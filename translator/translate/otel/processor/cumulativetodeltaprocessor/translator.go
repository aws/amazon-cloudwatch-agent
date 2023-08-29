// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cumulativetodeltaprocessor

import (
	"fmt"

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
)

type translator struct {
	name    string
	factory processor.Factory
}

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslator() common.Translator[component.Config] {
	return NewTranslatorWithName("")
}

func NewTranslatorWithName(name string) common.Translator[component.Config] {
	return &translator{name, cumulativetodeltaprocessor.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates a processor config based on the fields in the
// Metrics section of the JSON config.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || (!conf.IsSet(diskioKey) && !conf.IsSet(netKey)) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: fmt.Sprint(diskioKey, " or ", netKey)}
	}

	cfg := t.factory.CreateDefaultConfig().(*cumulativetodeltaprocessor.Config)

	excludeMetrics := t.getExcludeNetAndDiskIOMetrics(conf)

	if len(excludeMetrics) != 0 {
		cfg.Exclude.MatchType = strict
		cfg.Exclude.Metrics = excludeMetrics
	}
	return cfg, nil
}

// DiskIO and Net Metrics are cumulative metrics
// DiskIO: https://github.com/shirou/gopsutil/blob/master/disk/disk.go#L32-L47
// Net: https://github.com/shirou/gopsutil/blob/master/net/net.go#L13-L25
// However, CloudWatch  does have an upper bound https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_PutMetricData.html
// Therefore, we calculate the delta values for customers instead of using the original values
// https://github.com/aws/amazon-cloudwatch-agent/blob/5ace5aa6d817684cf82f4e6aa82d9596fb56d74b/translator/translate/metrics/util/deltasutil.go#L33-L65
func (t *translator) getExcludeNetAndDiskIOMetrics(conf *confmap.Conf) []string {
	var excludeMetricName []string
	if conf.IsSet(diskioKey) {
		excludeMetricName = append(excludeMetricName, "iops_in_progress", "diskio_iops_in_progress")
	}
	return excludeMetricName
}
