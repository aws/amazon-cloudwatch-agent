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

		// Historically, for Prometheus pipelines, we use the "job" corresponding to the target in the prometheus config as the log stream name
		// https://github.com/aws/amazon-cloudwatch-agent/blob/59cfe656152e31ca27e7983fac4682d0c33d3316/plugins/inputs/prometheus_scraper/metrics_handler.go#L80-L84
		// As can be seen, if the "job" tag was available, the log_stream_name would be set to it and if it wasnt available for some reason, the log_stream_name would be set as "default".
		// The old cloudwatchlogs exporter had logic to look for log_stream_name and if not found, it would use the log_stream_name defined in the config
		// https://github.com/aws/amazon-cloudwatch-agent/blob/60ca11244badf0cb3ae9dd9984c29f41d7a69302/plugins/outputs/cloudwatchlogs/cloudwatchlogs.go#L175-L180
		// But as we see above, there should never be a case for Prometheus pipelines where log_stream_name wasnt being set in metrics_handler - so the log_stream_name in the config would have never been used.

		// Now that we have switched to awsemfexporter, we leverage the token replacement logic to dynamically set the log stream name
		// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/897db04f747f0bda1707c916b1ec9f6c79a0c678/exporter/awsemfexporter/util.go#L29-L37
		// Hence we always set the log stream name in the default exporter config as {JobName} during config translation.
		// If we have a "job" tag, we do NOT add a tag for "JobName" here since the fallback logic in awsemfexporter while doing pattern matching will fallback from "JobName" -> "job" and use that.
		// Only when "job" tag isnt available, we set the "JobName" tag to default to retain same logic as before.
		// We do it this way so we dont unnecessarily add an extra tag (that the awsemfexporter wont know to drop) for most cases where "job" will be defined.

		if _, ok := mm.tags["job"]; !ok {
			mm.tags["JobName"] = "default"
		}
	}
}
