// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package adapter

import (
	"context"
	"testing"
	"time"

	telegrafconfig "github.com/influxdata/telegraf/config"
	_ "github.com/influxdata/telegraf/plugins/inputs/cpu"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

func Test_Type(t *testing.T) {
	t.Helper()

	as := assert.New(t)

	c := telegrafconfig.NewConfig()
	err := c.LoadConfig("./testdata/cpu_plugin.toml")
	as.NoError(err)

	adapter := NewAdapter(c)
	factory := adapter.NewReceiverFactory("cpu")
	ft := factory.Type()
	as.Equal(config.Type("telegraf_cpu"), ft)
}

func Test_ValidConfig(t *testing.T) {
	t.Helper()

	as := assert.New(t)

	c := telegrafconfig.NewConfig()
	err := c.LoadConfig("./testdata/cpu_plugin.toml")
	as.NoError(err)

	adapter := NewAdapter(c)
	factory := adapter.NewReceiverFactory("cpu")
	err = factory.CreateDefaultConfig().Validate()
	as.NoError(err)
}

func Test_CreateMetricsReceiver(t *testing.T) {
	t.Helper()

	as := assert.New(t)

	c := telegrafconfig.NewConfig()
	err := c.LoadConfig("./testdata/cpu_plugin.toml")
	as.NoError(err)

	adapter := NewAdapter(c)

	factory := adapter.NewReceiverFactory("cpu")
	metricsReceiver, err := factory.CreateMetricsReceiver(
		context.Background(),
		componenttest.NewNopReceiverCreateSettings(),
		&Config{
			ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
				CollectionInterval: time.Minute,
			},
		},
		consumertest.NewNop(),
	)
	as.NoError(err)
	as.NotNil(metricsReceiver)
}

func Test_CreateInvalidMetricsReceiver(t *testing.T) {
	t.Helper()

	as := assert.New(t)

	c := telegrafconfig.NewConfig()
	err := c.LoadConfig("./testdata/cpu_plugin.toml")
	as.NoError(err)

	adapter := NewAdapter(c)

	factory := adapter.NewReceiverFactory("mem")
	metricsReceiver, err := factory.CreateMetricsReceiver(
		context.Background(),
		componenttest.NewNopReceiverCreateSettings(),
		&Config{
			ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
				CollectionInterval: time.Minute,
			},
		},
		consumertest.NewNop(),
	)
	as.Error(err)
	as.Nil(metricsReceiver)
}
