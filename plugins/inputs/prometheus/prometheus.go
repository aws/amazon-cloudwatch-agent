// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	_ "embed"
	"sync"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"

	"github.com/aws/amazon-cloudwatch-agent/internal/ecsservicediscovery"
)

//go:embed prometheus.toml
var sampleConfig string

type Prometheus struct {
	PrometheusConfigPath string                                      `toml:"prometheus_config_path"`
	ClusterName          string                                      `toml:"cluster_name"`
	ECSSDConfig          *ecsservicediscovery.ServiceDiscoveryConfig `toml:"ecs_service_discovery"`
	mbCh                 chan PrometheusMetricBatch
	shutDownChan         chan interface{}
	wg                   sync.WaitGroup
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
	handler := &metricsHandler{mbCh: p.mbCh,
		acc:         accIn,
		calculator:  NewCalculator(),
		filter:      NewMetricsFilter(),
		clusterName: p.ClusterName,
		mtHandler:   mth,
	}

	ecssd := &ecsservicediscovery.ServiceDiscovery{Config: p.ECSSDConfig}

	// Start ECS Service Discovery when in ECS
	// https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/ContainerInsights-Prometheus-Setup-autodiscovery-ecs.html
	p.wg.Add(1)
	go ecsservicediscovery.StartECSServiceDiscovery(ecssd, p.shutDownChan, &p.wg)

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
		}
	})
}
