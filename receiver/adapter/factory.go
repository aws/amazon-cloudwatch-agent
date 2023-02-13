// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package adapter

import (
	"context"
	"fmt"
	"time"

	telegrafconfig "github.com/influxdata/telegraf/config"
	"github.com/influxdata/telegraf/models"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

const (
	TelegrafPrefix = "telegraf_"
)

type Adapter struct {
	telegrafConfig *telegrafconfig.Config
}

func NewAdapter(telegrafConfig *telegrafconfig.Config) Adapter {
	return Adapter{
		telegrafConfig: telegrafConfig,
	}
}

// Type joins the TelegrafPrefix to the input.
func Type(input string) component.Type {
	return component.Type(TelegrafPrefix + input)
}

func createDefaultConfig(cfgType component.Type) func() component.Config {
	return func() component.Config {
		return &Config{
			ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
				ReceiverSettings:   config.NewReceiverSettings(component.NewID(cfgType)),
				CollectionInterval: time.Minute,
			},
		}
	}
}

func (a Adapter) NewReceiverFactory(telegrafInputName string) receiver.Factory {
	typeStr := Type(telegrafInputName)
	return component.NewReceiverFactory(typeStr, createDefaultConfig(typeStr),
		component.WithMetricsReceiver(a.createMetricsReceiver(telegrafInputName), component.StabilityLevelStable))
}

func (a Adapter) createMetricsReceiver(telegrafInputName string) func(ctx context.Context, settings component.ReceiverCreateSettings, config component.Config, consumer consumer.Metrics) (component.MetricsReceiver, error) {
	input, err := a.initializeInput(telegrafInputName)
	return func(_ context.Context, settings component.ReceiverCreateSettings, rConf component.Config, consumer consumer.Metrics) (component.MetricsReceiver, error) {
		cfg := rConf.(*Config)

		if err != nil {
			return nil, err
		}

		receiver := newAdaptedReceiver(input, settings.Logger)

		scraper, err := scraperhelper.NewScraper(
			telegrafInputName,
			receiver.scrape,
			scraperhelper.WithStart(receiver.start),
			scraperhelper.WithShutdown(receiver.shutdown),
		)

		if err != nil {
			return nil, err
		}

		return scraperhelper.NewScraperControllerReceiver(
			&cfg.ScraperControllerSettings, settings, consumer,
			scraperhelper.AddScraper(scraper),
		)
	}
}

func (a Adapter) initializeInput(telegrafInputName string) (*models.RunningInput, error) {
	for _, ri := range a.telegrafConfig.Inputs {
		if ri.Config.Name == telegrafInputName {
			// Initialize the telegraf plugins to set value https://github.com/influxdata/telegraf/blob/3b3584b40b7c9ea10ae9cb02137fc072da202704/agent/agent.go#L197-L202
			// E.g Mem scrape their metrics based on OS https://github.com/influxdata/telegraf/blob/3b3584b40b7c9ea10ae9cb02137fc072da202704/plugins/inputs/mem/mem.go#L26-L29
			// and Init to set the Runtime OS
			err := ri.Init()
			if err != nil {
				return nil, fmt.Errorf("could not initialize input %s: %v", ri.LogName(), err)
			}

			return ri, nil
		}

	}

	return nil, fmt.Errorf("unable to find telegraf input with name %s", telegrafInputName)
}
