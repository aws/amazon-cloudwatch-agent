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
	"github.com/aws/amazon-cloudwatch-agent/internal/ecsservicediscovery"
)

//go:embed prometheus.toml
var sampleConfig string

type Prometheus struct {
	PrometheusConfigPath string                                      `toml:"prometheus_config_path"`
	ClusterName          string                                      `toml:"cluster_name"`
	ECSSDConfig          *ecsservicediscovery.ServiceDiscoveryConfig `toml:"ecs_service_discovery"`
	OtelECSObserverConfig *ecsobserver.Config                        `toml:"otel_ecs_observer"`
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

	var configurer *awsmiddleware.Configurer
	var ecssd *ecsservicediscovery.ServiceDiscovery
	needEcssd := true

	if p.middleware != nil {
		configurer = awsmiddleware.NewConfigurer(p.middleware.Handlers())
		if configurer != nil {
			ecssd = &ecsservicediscovery.ServiceDiscovery{Config: p.ECSSDConfig, Configurer: configurer}
			needEcssd = false
		}
	}
	if needEcssd {
		ecssd = &ecsservicediscovery.ServiceDiscovery{Config: p.ECSSDConfig}
	}

	// Validate OtelECSObserverConfig is not null when using OpenTelemetry pipeline
	if p.OtelECSObserverConfig == nil && p.ECSSDConfig == nil {
		// Neither config is provided, which is fine
	} else if p.OtelECSObserverConfig != nil {
		// Ensure the OtelECSObserverConfig has required fields
		if p.OtelECSObserverConfig.ClusterName == "" || p.OtelECSObserverConfig.ClusterRegion == "" || p.OtelECSObserverConfig.ResultFile == "" {
			return fmt.Errorf("OtelECSObserverConfig is missing required fields: ClusterName, ClusterRegion, or ResultFile")
		}
	}

	// Launch ECS Service Discovery as a goroutine
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
