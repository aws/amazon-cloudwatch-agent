// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package hostmetrics

import (
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	defaultCollectionInterval = time.Minute
)

// MetricType defines a hostmetrics metric type configuration.
type MetricType struct {
	Key         string // JSON configuration key
	ScraperName string // OpenTelemetry scraper name
}

// supportedMetrics defines all hostmetrics types supported by the translator.
// To add new metrics, append to this slice with the appropriate Key and ScraperName.
var supportedMetrics = []MetricType{
	{Key: "load", ScraperName: "load"},
}

// translator implements the hostmetrics receiver translator for CloudWatch agent.
type translator struct {
	common.NameProvider
	factory receiver.Factory
}

// NewTranslator creates a new hostmetrics receiver translator.
func NewTranslator(opts ...common.TranslatorOption) common.ComponentTranslator {
	t := &translator{factory: hostmetricsreceiver.NewFactory()}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// ID returns the component ID for this translator.
func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.Name())
}

// Translate converts CloudWatch agent configuration to OpenTelemetry hostmetrics receiver configuration.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	scrapers, interval, err := t.buildScrapers(conf)
	if err != nil {
		if missingKeyErr, ok := err.(*common.MissingKeyError); ok {
			missingKeyErr.ID = t.ID()
		}
		return nil, err
	}

	return map[string]interface{}{
		"collection_interval": interval.String(),
		"scrapers":           scrapers,
	}, nil
}

// buildScrapers dynamically builds scrapers configuration based on configured metrics.
func (t *translator) buildScrapers(conf *confmap.Conf) (map[string]interface{}, time.Duration, error) {
	baseConfigKey := common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey)
	scrapers := make(map[string]interface{})
	interval := defaultCollectionInterval

	for _, metric := range supportedMetrics {
		metricKey := common.ConfigKey(baseConfigKey, metric.Key)
		if conf.IsSet(metricKey) {
			scrapers[metric.ScraperName] = struct{}{}
			
			// Use the collection interval from the first configured metric
			if interval == defaultCollectionInterval {
				intervalKeyChain := []string{
					common.ConfigKey(metricKey, common.MetricsCollectionIntervalKey),
					common.ConfigKey(common.AgentKey, common.MetricsCollectionIntervalKey),
				}
				interval = common.GetOrDefaultDuration(conf, intervalKeyChain, defaultCollectionInterval)
			}
		}
	}

	if len(scrapers) == 0 {
		return nil, 0, &common.MissingKeyError{JsonKey: baseConfigKey}
	}

	return scrapers, interval, nil
}