// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"log"
	"sync"
	"time"

	"github.com/influxdata/telegraf"

	"github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
)

// Use metricMaterial instead of mbMetric to avoid unnecessary tags&fields copy
type metricMaterial struct {
	tags     map[string]string
	fields   map[string]interface{}
	timeInMS int64
}

type metricsHandler struct {
	mbCh        <-chan PrometheusMetricBatch
	acc         telegraf.Accumulator
	calculator  *Calculator
	filter      *MetricsFilter
	clusterName string
	mtHandler   *metricsTypeHandler
}

func (mh *metricsHandler) start(shutDownChan chan interface{}, wg *sync.WaitGroup) {
	for {
		select {
		case metricBatch := <-mh.mbCh:
			log.Printf("D! receive metric batch with %v prometheus metrics\n", len(metricBatch))
			mh.handle(metricBatch)
		case <-shutDownChan:
			wg.Done()
			return
		}
	}
}

func (mh *metricsHandler) handle(pmb PrometheusMetricBatch) {
	// Add metric type info
	pmb = mh.mtHandler.Handle(pmb)

	// Filter out Histogram and untyped Metrics and adding logging
	pmb = mh.filter.Filter(pmb)

	// do calculation: calculate delta for counter
	pmb = mh.calculator.Calculate(pmb)

	// do merge: merge metrics which are sharing same tags
	metricMaterials := mergeMetrics(pmb)

	// set emf
	mh.setEmfMetadata(metricMaterials)

	for _, metricMaterial := range metricMaterials {
		mh.acc.AddFields("prometheus", metricMaterial.fields, metricMaterial.tags, time.UnixMilli(metricMaterial.timeInMS))
	}
}

// set timestamp, version, logstream
func (mh *metricsHandler) setEmfMetadata(mms []*metricMaterial) {
	for _, mm := range mms {
		if mh.clusterName != "" {
			// Customer can specified the cluster name in the scraping job's relabel_config
			// CWAgent won't overwrite in this case to support cross-cluster monitoring
			if _, ok := mm.tags[containerinsightscommon.ClusterNameKey]; !ok {
				mm.tags[containerinsightscommon.ClusterNameKey] = mh.clusterName
			}
		}

		// Prometheus will use the "job" corresponding to the target in prometheus as a log stream
		// https://github.com/aws/amazon-cloudwatch-agent/blob/59cfe656152e31ca27e7983fac4682d0c33d3316/plugins/inputs/prometheus_scraper/metrics_handler.go#L80-L84
		// While determining the target, we would give preference to the metric tag over the log_stream_name coming from config/toml as per
		// https://github.com/aws/amazon-cloudwatch-agent/blob/60ca11244badf0cb3ae9dd9984c29f41d7a69302/plugins/outputs/cloudwatchlogs/cloudwatchlogs.go#L175-L180.

		// However, since we are using awsemfexport, we can leverage the token replacement with the log stream name
		// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/897db04f747f0bda1707c916b1ec9f6c79a0c678/exporter/awsemfexporter/util.go#L29-L37
		// Therefore, add a tag {ServiceName} for replacing job as a log stream

		if job, ok := mm.tags["job"]; ok {
			mm.tags["ServiceName"] = job
		} else {
			mm.tags["ServiceName"] = "default"
		}
	}
}
