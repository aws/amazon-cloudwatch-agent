// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package hostmetrics

import (
	"fmt"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/memoryscraper"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type translator struct {
	factory receiver.Factory
	common.NameProvider
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator(
	opts ...common.TranslatorOption,
) common.ComponentTranslator {
	t := &translator{factory: hostmetricsreceiver.NewFactory()}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.Name())
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: ""}
	}

	cfg := t.factory.CreateDefaultConfig().(*hostmetricsreceiver.Config)

	intervalKeyChain := []string{
		common.MetricsKey, common.MetricsCollectedKey, common.MemKey, common.MetricsCollectionIntervalKey,
	}
	if collectionInterval, ok := common.GetDuration(conf, common.ConfigKey(intervalKeyChain...)); ok {
		cfg.ControllerConfig.CollectionInterval = collectionInterval
	}

	// Configure memory scraper based on requested measurements
	memConfig := conf.Get(common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.MemKey))
	if memConfig == nil {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.MemKey)}
	}

	memConfigMap, ok := memConfig.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid mem configuration format")
	}

	measurements := common.GetMeasurements(memConfigMap)
	if len(measurements) == 0 {
		return nil, fmt.Errorf("no measurements configured for mem")
	}

	// Build metrics configuration based on measurements
	metrics := make(map[string]any)
	for _, measurement := range measurements {
		// Only include hostmetrics memory metrics. We do not want any Telegraf metrics here
		if IsHostmetricsMemoryMetric(measurement) {
			otelMetricName := getOtelMetricName(measurement)
			if otelMetricName != "" {
				metrics[otelMetricName] = map[string]any{
					"enabled": true,
				}
			}
		}
	}

	if len(metrics) == 0 {
		return nil, fmt.Errorf("no hostmetrics memory metrics found in measurements")
	}

	// Configure memory scraper
	memoryScraperConfig := map[string]any{
		"metrics": metrics,
	}

	c := confmap.NewFromStringMap(memoryScraperConfig)
	memoryFactory := getMemoryScraperFactory()
	memoryCfg := memoryFactory.CreateDefaultConfig()
	if err := c.Unmarshal(memoryCfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal memory scraper config (%s): %w", t.ID(), err)
	}

	cfg.Scrapers = map[component.Type]component.Config{
		memoryFactory.Type(): memoryCfg,
	}

	return cfg, nil
}

// IsHostmetricsMemoryMetric returns true if the metric should be collected by hostmetrics receiver instead of Telegraf
func IsHostmetricsMemoryMetric(metricName string) bool {
	hostmetricsMemoryMetrics := []string{
		"shared", // mem_shared -> system.linux.memory.shared
	}
	
	// Remove mem_ prefix if present for comparison
	cleanMetricName := strings.TrimPrefix(metricName, "mem_")
	
	for _, metric := range hostmetricsMemoryMetrics {
		if cleanMetricName == metric {
			return true
		}
	}
	return false
}

// getOtelMetricName maps CloudWatch Agent memory metric names to OpenTelemetry metric names
func getOtelMetricName(cwMetricName string) string {
	metricMap := map[string]string{
		"shared":     "system.linux.memory.shared",
		"mem_shared": "system.linux.memory.shared",
	}
	
	if otelName, exists := metricMap[cwMetricName]; exists {
		return otelName
	}
	
	// Try without mem_ prefix
	cleanName := strings.TrimPrefix(cwMetricName, "mem_")
	if otelName, exists := metricMap[cleanName]; exists {
		return otelName
	}
	
	return ""
}

// getMemoryScraperFactory returns the memory scraper factory
func getMemoryScraperFactory() scraper.Factory {
	return memoryscraper.NewFactory()
}nTelemetry metric for a CloudWatch Agent measurement
func (t *translator) enableMetricForMeasurement(cwMetricType, measurement string, scraperConfig map[string]interface{}, defaultConfig ScraperConfig) error {
	// Get metric mappings for this CloudWatch Agent metric type
	metricMappings, exists := scraperMetricMappings[cwMetricType]
	if !exists {
		return fmt.Errorf("no metric mappings found for CloudWatch Agent metric type: %s", cwMetricType)
	}
	
	// Find the OpenTelemetry metric for this measurement
	otlpMetric, exists := metricMappings[measurement]
	if !exists {
		// If no direct mapping, try to enable a reasonable default
		return fmt.Errorf("no OpenTelemetry metric mapping found for measurement: %s", measurement)
	}
	
	// Enable the metric
	metricsConfig := scraperConfig["metrg]interface{})
	metricsConfig[otlpMetric] = map[string]interface{}{
		"enabled": true,
	}
	
	return nil
}