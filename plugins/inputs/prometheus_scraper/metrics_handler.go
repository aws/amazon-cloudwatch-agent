package prometheus_scraper

import (
	"log"
	"strconv"
	"sync"

	"github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"github.com/aws/amazon-cloudwatch-agent/internal/logscommon"
	"github.com/influxdata/telegraf"
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
	// Filter out Summary, Histogram and untyped Metrics and adding logging
	pmb = mh.filter.Filter(pmb)

	// do calculation: calculate delta for counter
	pmb = mh.calculator.Calculate(pmb)

	// do merge: merge metrics which are sharing same tags
	metricMaterials := mergeMetrics(pmb)

	// set emf
	mh.setEmfMetadata(metricMaterials)

	for _, metricMaterial := range metricMaterials {
		mh.acc.AddFields("prometheus_scraper", metricMaterial.fields, metricMaterial.tags)
	}
}

// set timestamp, version, logstream
func (mh *metricsHandler) setEmfMetadata(mms []*metricMaterial) {
	for _, mm := range mms {
		mm.tags[logscommon.TimestampTag] = strconv.FormatInt(mm.timeInMS, 10)
		mm.tags[logscommon.VersionTag] = "0"

		if mh.clusterName != "" {
			// Customer can specified the cluster name in the scraping job's relabel_config
			// CWAgent won't overwrite in this case to support cross-cluster monitoring
			if _, ok := mm.tags[containerinsightscommon.ClusterNameKey]; !ok {
				mm.tags[containerinsightscommon.ClusterNameKey] = mh.clusterName
			}
		}

		if job, ok := mm.tags["job"]; ok {
			mm.tags[logscommon.LogStreamNameTag] = job
		} else {
			mm.tags[logscommon.LogStreamNameTag] = "default"
		}
	}
}
