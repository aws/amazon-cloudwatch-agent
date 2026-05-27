// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package hostmetrics

import (
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

var defaultScrapers = []string{"cpu", "disk", "filesystem", "memory", "network", "load", "processes"}

// hostMetricsConfig is a serializable representation of the hostmetrics receiver
// config. The upstream Config uses mapstructure:"-" on Scrapers which prevents
// serialization, so we define our own struct with an explicit scrapers field.
type hostMetricsConfig struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	Scrapers                       map[string]map[string]any `mapstructure:"scrapers"`
}

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

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	scrapers := make(map[string]map[string]any, len(defaultScrapers))
	for _, s := range defaultScrapers {
		scrapers[s] = map[string]any{}
	}
	return &hostMetricsConfig{
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: 60 * time.Second,
			InitialDelay:       time.Second,
		},
		Scrapers: scrapers,
	}, nil
}
