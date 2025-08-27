// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package hostmetrics

import (
	"fmt"
	"log"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)



// Config for hostmetrics receiver that properly marshals to YAML
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	Scrapers                       map[string]interface{} `mapstructure:"scrapers" yaml:"scrapers"`
	RootPath                       string                 `mapstructure:"root_path" yaml:"root_path"`
	MetadataCollectionInterval     time.Duration          `mapstructure:"metadata_collection_interval" yaml:"metadata_collection_interval"`
}

func (c *Config) Validate() error {
	if len(c.Scrapers) == 0 {
		return fmt.Errorf("must specify at least one scraper when using hostmetrics receiver")
	}
	return nil
}

type translator struct {
	name    string
	factory receiver.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator() common.ComponentTranslator {
	return NewTranslatorWithName("")
}

func NewTranslatorWithName(name string) common.ComponentTranslator {
	return &translator{name, hostmetricsreceiver.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	// Check if CPU configuration exists
	if !conf.IsSet(common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, "cpu", "measurement")) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: "cpu.measurement"}
	}

	cpuConfig := conf.Get(common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, "cpu"))
	if cpuConfig == nil {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: "cpu"}
	}

	measurements := common.GetMeasurements(cpuConfig.(map[string]any))

	// Simple configuration: always use cpu-avg dimension with per-core load average
	cpuAverageFlag := true // Divide load average by number of cores

	// Get collection interval from CPU configuration (inherit from CPU category)
	intervalKeyChain := []string{
		common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, "cpu", common.MetricsCollectionIntervalKey),
		common.ConfigKey(common.AgentKey, common.MetricsCollectionIntervalKey),
	}
	collectionInterval := common.GetOrDefaultDuration(conf, intervalKeyChain, time.Minute)

	// Determine which load average metrics to enable based on measurements
	loadMetricsConfig := getLoadMetricsConfig(measurements)

	// Debug: Log which metrics are enabled
	for metricName, config := range loadMetricsConfig {
		if configMap, ok := config.(map[string]interface{}); ok {
			if enabled, ok := configMap["enabled"].(bool); ok && enabled {
				log.Printf("D! hostmetrics load metric enabled: %s", metricName)
			}
		}
	}

	// Create our config with proper YAML marshaling
	config := &Config{
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: collectionInterval,
			InitialDelay:       time.Second,
		},
		Scrapers: map[string]interface{}{
			"load": map[string]interface{}{
				"cpu_average": cpuAverageFlag,
				"metrics":     loadMetricsConfig,
			},
		},
		MetadataCollectionInterval: 5 * time.Minute,
	}

	// Debug: Log the configuration values
	log.Printf("D! hostmetrics collection_interval configured: %s", collectionInterval.String())
	log.Printf("D! hostmetrics cpu_average flag: %t", cpuAverageFlag)
	log.Printf("D! hostmetrics config created: %+v", config)

	return config, nil
}

// getLoadMetricsConfig returns a metrics configuration that disables all load metrics by default
// and only enables the ones that are actually requested in the measurements
func getLoadMetricsConfig(measurements []string) map[string]interface{} {
	// Define all available load metrics - start with all disabled
	allLoadMetrics := []string{
		"system.cpu.load_average.1m",
		"system.cpu.load_average.5m",
		"system.cpu.load_average.15m",
	}

	// Start with all metrics disabled
	metricsConfig := make(map[string]interface{})
	for _, metric := range allLoadMetrics {
		metricsConfig[metric] = map[string]interface{}{
			"enabled": false,
		}
	}

	// Enable only the metrics that are requested
	for _, measurement := range measurements {
		switch measurement {
		case "cpu_load_average", "load_average":
			// For cpu_load_average or load_average, we only want the 1-minute metric
			metricsConfig["system.cpu.load_average.1m"] = map[string]interface{}{
				"enabled": true,
			}
		}
	}

	return metricsConfig
}

// IsHostmetricsMetric checks if a given metric name should be handled by the hostmetrics receiver
func IsHostmetricsMetric(metricName string) bool {
	// Load average metrics only
	return metricName == "load_average" || metricName == "cpu_load_average"
}