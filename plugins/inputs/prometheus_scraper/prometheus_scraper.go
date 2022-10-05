// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus_scraper

import (
	"sync"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/internal/ecsservicediscovery"
)

type PrometheusScraper struct {
	PrometheusConfigPath string                                      `toml:"prometheus_config_path"`
	ClusterName          string                                      `toml:"cluster_name"`
	ECSSDConfig          *ecsservicediscovery.ServiceDiscoveryConfig `toml:"ecs_service_discovery"`
	mbCh                 chan PrometheusMetricBatch
	shutDownChan         chan interface{}
	wg                   sync.WaitGroup
}

const sampleConfig = `
  [[inputs.prometheus_scraper]]
    cluster_name = "EC2-EC2-Justin-Testing"
    prometheus_config_path = "/opt/aws/amazon-cloudwatch-agent/etc/prometheus.yaml"
    [inputs.prometheus_scraper.ecs_service_discovery]
      sd_cluster_region = "us-east-2"
      sd_frequency = "15s"
      sd_result_file = "/opt/aws/amazon-cloudwatch-agent/etc/ecs_sd_targets.yaml"
      sd_target_clusters = "EC2-EC2-Justin-Testing"
      [inputs.prometheus_scraper.ecs_service_discovery.docker_label]
        sd_job_name_label = "ECS_PROMETHEUS_JOB_NAME_1"
        sd_metrics_path_label = "ECS_PROMETHEUS_METRICS_PATH"
        sd_port_label = "ECS_PROMETHEUS_EXPORTER_PORT_SUBSET"

      [[inputs.prometheus_scraper.ecs_service_discovery.task_definition_list]]
        sd_job_name = "task_def_1"
        sd_metrics_path = "/stats/metrics"
        sd_metrics_ports = "9901"
        sd_task_definition_name = "task_def_1"

      [[inputs.prometheus_scraper.ecs_service_discovery.task_definition_list]]
        sd_metrics_ports = "9902"
        sd_task_definition_name = "task_def_2"
    [inputs.prometheus_scraper.tags]
      metricPath = "logs"
`

func (p *PrometheusScraper) SampleConfig() string {
	return sampleConfig
}

func (p *PrometheusScraper) Description() string {
	return "PrometheusScraper is used to scrape metrics from prometheus exporter"
}

func (p *PrometheusScraper) Gather(_ telegraf.Accumulator) error {
	return nil
}

func (p *PrometheusScraper) Start(accIn telegraf.Accumulator) error {
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

	// start ECS Service Discovery
	p.wg.Add(1)
	go ecsservicediscovery.StartECSServiceDiscovery(ecssd, p.shutDownChan, &p.wg)

	// start metric collecting
	p.wg.Add(1)
	go Start(p.PrometheusConfigPath, receiver, p.shutDownChan, &p.wg, mth)

	// start metric handling
	p.wg.Add(1)
	go handler.start(p.shutDownChan, &p.wg)

	return nil
}

func (p *PrometheusScraper) Stop() {
	close(p.shutDownChan)
	p.wg.Wait()
}

func init() {
	inputs.Add("prometheus_scraper", func() telegraf.Input {
		return &PrometheusScraper{mbCh: make(chan PrometheusMetricBatch, 10000),
			shutDownChan: make(chan interface{})}
	})
}
