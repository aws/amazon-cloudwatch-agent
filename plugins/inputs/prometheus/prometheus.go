// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	_ "embed"
	"fmt"
	"sync"

	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/observer/ecsobserver"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth"
)

//go:embed prometheus.toml
var sampleConfig string

type Prometheus struct {
	PrometheusConfigPath string              `toml:"prometheus_config_path"`
	ClusterName          string              `toml:"cluster_name"`
	ECSObserverConfig    *ecsobserver.Config `toml:"ecs_observer_config"`
	mbCh                 chan PrometheusMetricBatch
	shutDownChan         chan interface{}
	wg                   sync.WaitGroup
	middleware           awsmiddleware.Middleware
}

func (p *Prometheus) SampleConfig() string {
	return sampleConfig
}

func (p *Prometheus) Description() string {
	return "Prometheus is used to scrape metrics from prometheus exporter"
}

func (p *Prometheus) Gather(_ telegraf.Accumulator) error {
	return nil
}

func (p *Prometheus) Start(accIn telegraf.Accumulator) error {
	mth := NewMetricsTypeHandler()

	receiver := &metricsReceiver{pmbCh: p.mbCh}
	handler := &metricsHandler{
		mbCh:        p.mbCh,
		acc:         accIn,
		calculator:  NewCalculator(),
		filter:      NewMetricsFilter(),
		clusterName: p.ClusterName,
		mtHandler:   mth,
	}

	// Validate ECSObserverConfig if provided
	if p.ECSObserverConfig != nil {
		// Ensure the ECSObserverConfig has required fields
		if p.ECSObserverConfig.ClusterName == "" || p.ECSObserverConfig.ClusterRegion == "" || p.ECSObserverConfig.ResultFile == "" {
			return fmt.Errorf("ECSObserverConfig is missing required fields: ClusterName, ClusterRegion, or ResultFile")
		}
		
		// Here we would initialize and start the ecsobserver
		// This would be implemented in the OpenTelemetry pipeline
		// rather than directly in this Telegraf-based code
	}

	// Start scraping prometheus metrics from prometheus endpoints
	p.wg.Add(1)
	go Start(p.PrometheusConfigPath, receiver, p.shutDownChan, &p.wg, mth)

	// Start filter our prometheus metrics, calculate delta value if its a Counter or Summary count sum
	// and convert Prometheus metrics to Telegraf Metrics
	p.wg.Add(1)
	go handler.start(p.shutDownChan, &p.wg)

	return nil
}

func (p *Prometheus) Stop() {
	close(p.shutDownChan)
	p.wg.Wait()
}

func init() {
	inputs.Add("prometheus", func() telegraf.Input {
		return &Prometheus{
			mbCh:         make(chan PrometheusMetricBatch, 10000),
			shutDownChan: make(chan interface{}),
			middleware: agenthealth.NewAgentHealth(
				zap.NewNop(),
				&agenthealth.Config{
					IsUsageDataEnabled:  envconfig.IsUsageDataEnabled(),
					IsStatusCodeEnabled: true,
				},
			),
		}

	})
}
