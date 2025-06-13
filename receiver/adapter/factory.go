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
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	otelscraper "go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
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
	newType, _ := component.NewType(TelegrafPrefix + input)
	return newType
}

func createDefaultConfig() func() component.Config {
	return func() component.Config {
		return &Config{
			ControllerConfig: scraperhelper.ControllerConfig{
				CollectionInterval: time.Minute,
			},
		}
	}
}

func (a Adapter) NewReceiverFactory(telegrafInputName string) receiver.Factory {
	typeStr := Type(telegrafInputName)
	return receiver.NewFactory(typeStr, createDefaultConfig(),
		receiver.WithMetrics(a.createMetricsReceiver, component.StabilityLevelStable))
}

func (a Adapter) createMetricsReceiver(ctx context.Context, settings receiver.Settings, config component.Config, consumer consumer.Metrics) (receiver.Metrics, error) {
	cfg := config.(*Config)
	input, err := a.initializeInput(settings.ID.Type().String(), settings.ID.Name())

	if err != nil {
		return nil, err
	}

	rcvr := newAdaptedReceiver(input, ctx, consumer, settings.Logger)

	scraper, err := otelscraper.NewMetrics(
		rcvr.scrape,
		otelscraper.WithStart(rcvr.start),
		otelscraper.WithShutdown(rcvr.shutdown),
	)

	if err != nil {
		return nil, err
	}

	return scraperhelper.NewMetricsController(
		&cfg.ControllerConfig, settings, consumer,
		scraperhelper.AddScraper(settings.ID.Type(), scraper),
	)
}

// initializeInput initialize the telegraf plugins to set value https://github.com/influxdata/telegraf/blob/3b3584b40b7c9ea10ae9cb02137fc072da202704/agent/agent.go#L197-L202
// E.g Mem scrape their metrics based on OS https://github.com/influxdata/telegraf/blob/3b3584b40b7c9ea10ae9cb02137fc072da202704/plugins/inputs/mem/mem.go#L26-L29
// and Init to set the Runtime OS
func (a Adapter) initializeInput(pluginName, pluginAlias string) (*models.RunningInput, error) {
	for _, ri := range a.telegrafConfig.Inputs {
		if TelegrafPrefix+ri.Config.Name == pluginName && ri.Config.Alias == pluginAlias {

			err := ri.Init()
			if err != nil {
				return nil, fmt.Errorf("could not initialize input %s: %v", ri.LogName(), err)
			}

			return ri, nil
		}

	}

	return nil, fmt.Errorf("unable to find telegraf input with name %s and alias %s", pluginName, pluginAlias)
}
