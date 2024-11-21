// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	_ "embed"
	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
	"go.uber.org/zap"
	"log"
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
	middleware           awsmiddleware.Middleware
	Configurer           *awsmiddleware.Configurer
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
	log.Println("Starting Prometheus")

	// Initialize Metrics Type Handler
	mth := NewMetricsTypeHandler()

	// Initialize the Prometheus receiver and handler
	receiver := &metricsReceiver{pmbCh: p.mbCh}
	handler := &metricsHandler{
		mbCh:        p.mbCh,
		acc:         accIn,
		calculator:  NewCalculator(),
		filter:      NewMetricsFilter(),
		clusterName: p.ClusterName,
		mtHandler:   mth,
	}

	var configurer *awsmiddleware.Configurer
	var ecssd *ecsservicediscovery.ServiceDiscovery

	if p.middleware != nil {
		if configurer = awsmiddleware.NewConfigurer(p.middleware.Handlers()); configurer != nil {
			log.Println("failed to configure awsmiddleware")
			ecssd = &ecsservicediscovery.ServiceDiscovery{Config: p.ECSSDConfig}

		} else {
			ecssd = &ecsservicediscovery.ServiceDiscovery{Config: p.ECSSDConfig, Configurer: configurer}
			log.Println("passed awsmiddleware configurer")
		}
	}

	// Launch ECS Service Discovery as a goroutine
	p.wg.Add(1)
	go ecsservicediscovery.StartECSServiceDiscovery(ecssd, p.shutDownChan, &p.wg)

	// Launch the Prometheus scraping process as a goroutine
	p.wg.Add(1)
	go Start(p.PrometheusConfigPath, receiver, p.shutDownChan, &p.wg, mth)

	// Launch the handler for filtering and converting Prometheus metrics as a goroutine
	p.wg.Add(1)
	go handler.start(p.shutDownChan, &p.wg)

	return nil
}

func (p *Prometheus) Stop() {
	close(p.shutDownChan)
	p.wg.Wait()
}

func init() {
	log.Println("Initializing Prometheus")
	inputs.Add("prometheus", func() telegraf.Input {
		boolean := true
		return &Prometheus{
			mbCh:         make(chan PrometheusMetricBatch, 10000),
			shutDownChan: make(chan interface{}),
			middleware: agenthealth.NewAgentHealth(
				zap.NewNop(),
				&agenthealth.Config{
					IsUsageDataEnabled: envconfig.IsUsageDataEnabled(),
					Stats:              agent.StatsConfig{Operations: []string{"PutMetricData"}},
					StatusCodeOnly:     &boolean,
				},
			),
		}

	})
}
