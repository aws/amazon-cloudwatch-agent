// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package adapter

import (
	"context"
	"fmt"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/tocwconfig/toyamlconfig"
	telegrafconfig "github.com/influxdata/telegraf/config"
	"github.com/influxdata/telegraf/models"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"time"
)

type Adapter struct {
	telegrafConfig *telegrafconfig.Config
}

func NewAdapter(telegrafConfig *telegrafconfig.Config) Adapter {
	return Adapter{
		telegrafConfig: telegrafConfig,
	}
}

func createDefaultConfig(cfgType config.Type) func() config.Receiver {
	return func() config.Receiver {
		return &Config{
			ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
				ReceiverSettings:   config.NewReceiverSettings(config.NewComponentID(cfgType)),
				CollectionInterval: time.Minute,
			},
		}
	}
}

func (a Adapter) NewReceiverFactory(telegrafInputName string) component.ReceiverFactory {
	typeStr := config.Type(toyamlconfig.TelegrafPrefix + telegrafInputName)
	return component.NewReceiverFactory(typeStr, createDefaultConfig(typeStr),
		component.WithMetricsReceiver(a.createMetricsReceiver(telegrafInputName), component.StabilityLevelStable))
}

func (a Adapter) createMetricsReceiver(telegrafInputName string) func(ctx context.Context, settings component.ReceiverCreateSettings, config config.Receiver, consumer consumer.Metrics) (component.MetricsReceiver, error) {
	input, err := a.initializeInput(telegrafInputName)
	return func(ctx context.Context, settings component.ReceiverCreateSettings, rConf config.Receiver, consumer consumer.Metrics) (component.MetricsReceiver, error) {
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

			// To Do: Add Service Input Start when collecting statsd, collectd metrics,.. moreover, signaling to set
			// different settings (e.g precision) which is different from regular inputs
			// https://github.com/influxdata/telegraf/blob/8c49ddccc3cb8f8fe020dc4e1f38b93a0f2ad467/agent/agent.go#L252-L274
			return ri, nil
		}

	}

	return nil, fmt.Errorf("unable to find telegraf input with name %s", telegrafInputName)
}
