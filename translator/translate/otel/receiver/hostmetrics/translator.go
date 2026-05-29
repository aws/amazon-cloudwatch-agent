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
	factory receiver.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator() common.ComponentTranslator {
	return &translator{factory: hostmetricsreceiver.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), "")
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	scrapers := make(map[string]map[string]any, len(defaultScrapers))
	for _, s := range defaultScrapers {
		scrapers[s] = nil
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
