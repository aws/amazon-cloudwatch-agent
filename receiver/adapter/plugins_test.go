// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package adapter

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/agent"
	"github.com/influxdata/telegraf/config"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// Service Input differs from a regular plugin in that it operates a background service while Telegraf/CWAgent is running
// https://github.com/influxdata/telegraf/blob/d67f75e55765d364ad0aabe99382656cb5b51014/docs/INPUTS.md#service-input-plugins
type regularInputConfig struct {
	scrapeCount int
}

type serviceInputConfig struct {
	protocol      string
	listeningPort string
	metricSending []byte
}

/*
sanityTestConfig struct
@plugin               Telegraf input plugins
@regularInputConfig   Telegraf Regular Input's Configuration including number of time scraping metrics
@serviceInputConfig   Telegraf Service Input's Configuration including the port, protocol, metric's format sending
*/

type sanityTestConfig struct {
	testConfig           string
	plugin               string
	regularInputConfig   regularInputConfig
	serviceInputConfig   serviceInputConfig
	expectedMetrics      [][]string
	numMetricsComparator assert.ComparisonAssertionFunc
}

func scrapeAndValidateMetrics(t *testing.T, cfg *sanityTestConfig) {
	as := assert.New(t)
	ctx := context.TODO()
	sink := new(consumertest.MetricsSink)
	receiver := getInitializedReceiver(as, cfg.plugin, cfg.testConfig, ctx, sink)

	err := receiver.start(ctx, nil)
	as.NoError(err)

	otelMetrics := scrapeMetrics(as, ctx, receiver, sink, cfg)

	err = receiver.shutdown(ctx)
	as.NoError(err)

	cfg.numMetricsComparator(t, len(cfg.expectedMetrics), otelMetrics.ResourceMetrics().Len())

	var metrics pmetric.MetricSlice
	for i := 0; i < len(cfg.expectedMetrics); i++ {
		metrics = otelMetrics.ResourceMetrics().At(i).ScopeMetrics().At(0).Metrics()
		validateMetricName(as, cfg.plugin, cfg.expectedMetrics[i], metrics)
	}
}

func getInitializedReceiver(as *assert.Assertions, plugin string, testConfig string, ctx context.Context, consumer consumer.Metrics) *AdaptedReceiver {
	c := config.NewConfig()
	c.InputFilters = []string{plugin}
	err := c.LoadConfig(testConfig)
	as.NoError(err)

	a, _ := agent.NewAgent(c)
	as.Len(a.Config.Inputs, 1)

	err = a.Config.Inputs[0].Init()
	as.NoError(err)

	return newAdaptedReceiver(a.Config.Inputs[0], ctx, consumer, zap.NewNop())
}

func scrapeMetrics(as *assert.Assertions, ctx context.Context, receiver *AdaptedReceiver, sink *consumertest.MetricsSink, cfg *sanityTestConfig) pmetric.Metrics {

	var err error
	var otelMetrics pmetric.Metrics

	if _, ok := receiver.input.Input.(telegraf.ServiceInput); ok {
		conn, err := net.Dial(cfg.serviceInputConfig.protocol, cfg.serviceInputConfig.listeningPort)
		as.NoError(err)
		_, err = conn.Write(cfg.serviceInputConfig.metricSending)
		as.NoError(err)
		as.NoError(conn.Close())

		for {
			otelMetrics = pmetric.NewMetrics()
			for _, metric := range sink.AllMetrics() {
				metric.ResourceMetrics().MoveAndAppendTo(otelMetrics.ResourceMetrics())
			}
			as.NoError(err)

			time.Sleep(time.Second)
			if otelMetrics.ResourceMetrics().Len() > 0 {
				break
			}
		}
	} else {
		for i := 0; i < cfg.regularInputConfig.scrapeCount; i++ {
			if i != 0 {
				time.Sleep(time.Second)
			}
			otelMetrics, err = receiver.scrape(ctx)
			as.NoError(err)
		}
	}

	return otelMetrics
}

func validateMetricName(as *assert.Assertions, plugin string, expectedResourceMetricsName []string, actualOtelSlMetrics pmetric.MetricSlice) {
	as.Equal(len(expectedResourceMetricsName), actualOtelSlMetrics.Len(), "Number of metrics did not match!")

	matchMetrics := actualOtelSlMetrics.Len()
	for _, expectedMetric := range expectedResourceMetricsName {
		for metricIndex := 0; metricIndex < actualOtelSlMetrics.Len(); metricIndex++ {
			metric := actualOtelSlMetrics.At(metricIndex)
			// Check name to decrease the match metrics since metric name is the only unique attribute
			// And ignore the rest checking
			if plugin == "win_perf_counters" {
				if expectedMetric != metric.Name() {
					continue
				}
			} else {
				if fmt.Sprintf("%s_%s", plugin, expectedMetric) != metric.Name() {
					continue
				}
			}
			matchMetrics--
		}
	}

	as.Equal(0, matchMetrics, "Metrics did not match!")
}
