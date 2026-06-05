// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package hostmetrics

import (
	_ "embed"
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"gopkg.in/yaml.v3"

	translatorconfig "github.com/aws/amazon-cloudwatch-agent/translator/config"
	translatorcontext "github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

//go:embed scrapers_linux.yaml
var scrapersLinuxConfig []byte

//go:embed scrapers_windows.yaml
var scrapersWindowsConfig []byte

// HostMetricsConfig is a serializable representation of
// hostmetricsreceiver.Config. The upstream type uses mapstructure:"-" on its
// Scrapers field which prevents confmap serialization, so we define our own
// struct with an explicit scrapers field.
type HostMetricsConfig struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	Scrapers                       map[string]map[string]any `mapstructure:"scrapers"`
}

// Validate is intentionally a no-op; the upstream hostmetricsreceiver handles its own validation.
func (c *HostMetricsConfig) Validate() error {
	return nil
}

type translator struct {
	factory        receiver.Factory
	processScraper map[string]any
}

type Option func(*translator)

func WithProcessScraper(scraperConfig map[string]any) Option {
	return func(t *translator) { t.processScraper = scraperConfig }
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

	// Select platform-specific scrapers config
	var scrapersYaml []byte
	if translatorcontext.CurrentContext().Os() == translatorconfig.OS_TYPE_WINDOWS {
		scrapersYaml = scrapersWindowsConfig
	} else {
		scrapersYaml = scrapersLinuxConfig
	}

	var scrapers map[string]map[string]any
	if err := yaml.Unmarshal(scrapersYaml, &scrapers); err != nil {
		return nil, fmt.Errorf("failed to parse scrapers config: %w", err)
	}

	// Add process scraper if DBI configured
	if t.processScraper != nil {
		scrapers["process"] = t.processScraper
	}

	intervalKeyChain := []string{
		common.ConfigKey(common.OpenTelemetryKey, common.CollectKey, common.HostInsightsKey, common.MetricsCollectionIntervalKey),
		common.ConfigKey(common.AgentKey, common.MetricsCollectionIntervalKey),
	}
	return &HostMetricsConfig{
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: common.GetOrDefaultDuration(conf, intervalKeyChain, 30*time.Second),
			InitialDelay:       time.Second,
		},
		Scrapers: scrapers,
	}, nil
}
