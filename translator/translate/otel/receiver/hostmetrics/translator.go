// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package hostmetrics

import (
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

var defaultScrapers = []string{"cpu", "disk", "filesystem", "memory", "network", "load", "processes"}

// hostMetricsConfig is a serializable representation of
// hostmetricsreceiver.Config. The upstream type uses mapstructure:"-" on its
// Scrapers field which prevents confmap serialization, so we define our own
// struct with an explicit scrapers field.
type hostMetricsConfig struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	Scrapers                       map[string]map[string]any `mapstructure:"scrapers"`
}

// Validate is intentionally a no-op; the upstream hostmetricsreceiver handles its own validation.
func (c *hostMetricsConfig) Validate() error {
	return nil
}

type translator struct {
	factory        receiver.Factory
	processScraper map[string]any
}

type Option func(*translator)

func WithProcessScraper(filter map[string]any) Option {
	return func(t *translator) { t.processScraper = filter }
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator(opts ...Option) common.ComponentTranslator {
	t := &translator{factory: hostmetricsreceiver.NewFactory()}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), "")
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil {
		conf = confmap.NewFromStringMap(map[string]interface{}{})
	}
	scrapers := map[string]map[string]any{
		"cpu": {
			"metrics": map[string]any{
				"system.cpu.frequency":      map[string]any{"enabled": true},
				"system.cpu.logical.count":  map[string]any{"enabled": true},
				"system.cpu.physical.count": map[string]any{"enabled": true},
				"system.cpu.utilization":    map[string]any{"enabled": true},
			},
		},
		"disk":    nil,
		"filesystem": {
			"metrics": map[string]any{
				"system.filesystem.utilization": map[string]any{"enabled": true},
			},
		},
		"load":    nil,
		"memory": {
			"metrics": map[string]any{
				"system.linux.memory.available": map[string]any{"enabled": true},
				"system.linux.memory.dirty":     map[string]any{"enabled": true},
				"system.memory.limit":           map[string]any{"enabled": true},
				"system.memory.page_size":       map[string]any{"enabled": true},
				"system.memory.utilization":     map[string]any{"enabled": true},
			},
		},
		"network": {
			"metrics": map[string]any{
				"system.network.conntrack.count": map[string]any{"enabled": true},
				"system.network.conntrack.max":   map[string]any{"enabled": true},
			},
		},
		"processes": nil,
	}
	if t.processScraper != nil {
		scrapers["process"] = t.processScraper
	}
	intervalKeyChain := []string{
		common.ConfigKey(common.OpenTelemetryKey, common.CollectKey, common.HostInsightsKey, common.MetricsCollectionIntervalKey),
		common.ConfigKey(common.AgentKey, common.MetricsCollectionIntervalKey),
	}
	return &hostMetricsConfig{
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: common.GetOrDefaultDuration(conf, intervalKeyChain, 60*time.Second),
			InitialDelay:       time.Second,
		},
		Scrapers: scrapers,
	}, nil
}
