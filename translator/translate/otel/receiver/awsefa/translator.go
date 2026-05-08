// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsefa

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsefareceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	defaultCollectionInterval = time.Minute
	efaPrefix                 = "efa_"
)

var (
	baseKey       = common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.EfaKey)
	allEfaMetrics = getAllEfaMetrics()
)

var _ common.ComponentTranslator = (*translator)(nil)

type translator struct {
	common.NameProvider
	factory receiver.Factory
}

func NewTranslator(opts ...common.TranslatorOption) common.ComponentTranslator {
	t := &translator{factory: awsefareceiver.NewFactory()}
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

	cfg := t.factory.CreateDefaultConfig().(*awsefareceiver.Config)

	intervalKeyChain := []string{
		common.ConfigKey(baseKey, common.MetricsCollectionIntervalKey),
		common.ConfigKey(common.AgentKey, common.MetricsCollectionIntervalKey),
	}
	cfg.CollectionInterval = common.GetOrDefaultDuration(conf, intervalKeyChain, defaultCollectionInterval)

	efaMap, ok := conf.Get(baseKey).(map[string]any)
	if !ok || efaMap == nil {
		return nil, fmt.Errorf("measurement is required for efa receiver (%s)", t.ID())
	}

	if _, hasMeasurement := efaMap[common.MeasurementKey]; !hasMeasurement {
		return nil, fmt.Errorf("measurement is required for efa receiver (%s)", t.ID())
	}

	measurements := common.GetMeasurements(efaMap)
	metrics := getEnabledMeasurements(measurements)
	c := confmap.NewFromStringMap(map[string]any{
		"metrics": metrics,
	})
	if err := c.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal efa receiver (%s): %w", t.ID(), err)
	}

	return cfg, nil
}

// getAllEfaMetrics derives the complete list of EFA metrics from the receiver's
// MetricsConfig struct via reflection, avoiding a hardcoded list that can go stale.
func getAllEfaMetrics() map[string]bool {
	cfg := awsefareceiver.NewFactory().CreateDefaultConfig().(*awsefareceiver.Config)
	metricsType := reflect.TypeOf(cfg.Metrics)
	result := make(map[string]bool, metricsType.NumField())
	for i := 0; i < metricsType.NumField(); i++ {
		tag := metricsType.Field(i).Tag.Get("mapstructure")
		if tag != "" {
			result[tag] = true
		}
	}
	return result
}

func getEnabledMeasurements(measurements []string) map[string]any {
	// Disable all metrics first.
	metrics := map[string]any{}
	for m := range allEfaMetrics {
		metrics[m] = map[string]any{"enabled": false}
	}
	// Enable only the selected ones that are valid EFA metrics.
	for _, m := range measurements {
		metricName := m
		if !strings.HasPrefix(m, efaPrefix) {
			metricName = efaPrefix + m
		}
		if allEfaMetrics[metricName] {
			metrics[metricName] = map[string]any{"enabled": true}
		}
	}
	return metrics
}
