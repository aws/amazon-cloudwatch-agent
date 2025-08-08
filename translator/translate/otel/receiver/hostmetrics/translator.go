// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package hostmetrics

import (
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	receiverType = "hostmetrics"
)

type translator struct {
	common.NameProvider
	cfgKey string
}

var _ common.ComponentTranslator = (*translator)(nil)

// NewTranslator creates a new hostmetrics receiver translator.
func NewTranslator(cfgKey string, opts ...common.TranslatorOption) common.ComponentTranslator {
	t := &translator{
		NameProvider: common.NameProvider{},
		cfgKey:       cfgKey,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(component.MustNewType(receiverType), t.Name())
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !conf.IsSet(t.cfgKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: t.cfgKey}
	}

	// Get load configuration
	loadConfig := conf.Get(t.cfgKey).(map[string]any)
	
	// Get measurements and validate them
	measurements := common.GetMeasurements(loadConfig)
	expectedMeasurements := map[string]bool{
		"load_average_1m":  true,
		"load_average_5m":  true,
		"load_average_15m": true,
	}
	for _, measurement := range measurements {
		if !expectedMeasurements[measurement] {
			return nil, fmt.Errorf("unsupported load measurement: %s. Supported measurements: load_average_1m, load_average_5m, load_average_15m", measurement)
		}
	}

	// Get collection interval - default to 60 seconds
	intervalKeyChain := []string{
		common.ConfigKey(t.cfgKey, common.MetricsCollectionIntervalKey),
		common.ConfigKey(common.AgentKey, common.MetricsCollectionIntervalKey),
	}
	collectionInterval := common.GetOrDefaultDuration(conf, intervalKeyChain, 60*time.Second)

	// Create selective metrics configuration based on requested measurements
	metricsConfig := map[string]any{}
	
	// Map JSON measurement names to OpenTelemetry metric names
	measurementToMetricName := map[string]string{
		"load_average_1m":  "system.cpu.load_average.1m",
		"load_average_5m":  "system.cpu.load_average.5m",
		"load_average_15m": "system.cpu.load_average.15m",
	}
	
	// Enable only the requested metrics
	requestedMetrics := make(map[string]bool)
	for _, measurement := range measurements {
		if metricName, exists := measurementToMetricName[measurement]; exists {
			requestedMetrics[metricName] = true
		}
	}
	
	// Configure all load metrics - enable only requested ones
	allLoadMetrics := []string{
		"system.cpu.load_average.1m",
		"system.cpu.load_average.5m", 
		"system.cpu.load_average.15m",
	}
	
	for _, metricName := range allLoadMetrics {
		metricsConfig[metricName] = map[string]any{
			"enabled": requestedMetrics[metricName], // true if requested, false otherwise
		}
	}

	// Create the complete configuration map including scrapers with selective metrics
	configMap := map[string]any{
		"collection_interval": collectionInterval,
		"scrapers": map[string]any{
			"load": map[string]any{
				"cpu_average": false, // We want total load, not per-CPU average
				"metrics":     metricsConfig,
			},
		},
	}
	
	// Create the hostmetrics receiver config
	factory := hostmetricsreceiver.NewFactory()
	cfg := factory.CreateDefaultConfig().(*hostmetricsreceiver.Config)
	
	// Create a confmap and unmarshal the complete configuration
	fullConf := confmap.NewFromStringMap(configMap)
	if err := fullConf.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal hostmetrics receiver config (%s): %w", t.ID(), err)
	}

	return cfg, nil
}