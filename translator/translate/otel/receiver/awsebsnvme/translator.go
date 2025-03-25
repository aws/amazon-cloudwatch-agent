// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsebsnvme

import (
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"

	"github.com/aws/amazon-cloudwatch-agent/receiver/awsebsnvmereceiver"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

var (
	baseKey = common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.DiskIOKey)
)

const (
	defaultCollectionInterval = time.Minute
	diskIOPrefix              = "diskio_"
	ebsPrefix                 = diskIOPrefix + "ebs_"
)

type translator struct {
	common.NameProvider
	factory receiver.Factory
}

func NewTranslator(
	opts ...common.TranslatorOption,
) common.ComponentTranslator {
	t := &translator{factory: awsebsnvmereceiver.NewFactory()}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.Name())
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !conf.IsSet(baseKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: baseKey}
	}

	cfg := t.factory.CreateDefaultConfig().(*awsebsnvmereceiver.Config)

	intervalKeyChain := []string{
		common.ConfigKey(baseKey, common.MetricsCollectionIntervalKey),
		common.ConfigKey(common.AgentKey, common.MetricsCollectionIntervalKey),
	}
	cfg.CollectionInterval = common.GetOrDefaultDuration(conf, intervalKeyChain, defaultCollectionInterval)

	resources := common.GetArray[string](conf, common.ConfigKey(baseKey, common.ResourcesKey))
	if resources == nil {
		// Was not set by the user, so collect all devices by default
		cfg.Resources = []string{"*"}
	} else {
		cfg.Resources = resources
	}

	// Total Read Ops is the only metric enabled by default. Disable it so that
	// the measurements from the agent config are used instead.
	cfg.MetricsBuilderConfig.Metrics.DiskioEbsTotalReadOps.Enabled = false
	c := confmap.NewFromStringMap(map[string]any{
		"metrics": getEnabledMeasurements(conf),
	})

	if err := c.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal ebs nvme receiver (%s): %w", t.ID(), err)
	}

	return cfg, nil
}

func getEnabledMeasurements(conf *confmap.Conf) map[string]any {
	measurements := common.GetMeasurements(conf.Get(baseKey).(map[string]any))

	metrics := map[string]any{}

	for _, m := range measurements {
		metricName := m
		if !strings.HasPrefix(m, diskIOPrefix) {
			metricName = diskIOPrefix + m
		}
		// Only include EBS metrics. We do not want any Telegraf metrics here
		if strings.HasPrefix(metricName, ebsPrefix) {
			metrics[metricName] = map[string]any{
				"enabled": true,
			}
		}
	}

	return metrics
}
