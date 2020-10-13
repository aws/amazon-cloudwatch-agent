package prometheus_scraper

import (
	"bytes"
	"fmt"
	"sort"
)

func getTagsKey(pm *PrometheusMetric) *bytes.Buffer {
	b := new(bytes.Buffer)
	keys := make([]string, 0, len(pm.tags))
	for k := range pm.tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		_, _ = fmt.Fprintf(b, "%s=%s,", k, pm.tags[k])
	}
	return b
}

// return MetricKey from all tags which is used to merge metrics which are sharing same tags
func getMetricKeyForMerging(pm *PrometheusMetric) string {
	return getTagsKey(pm).String()
}

// return uniq MetricKey, which is used to calculate delta of metrics.
func getUniqMetricKey(pm *PrometheusMetric) string {
	buffer := getTagsKey(pm)
	// We assume there won't be same metricName+tags with different metricType, so that it is not necessary to add metricType into uniqKey.
	_, _ = fmt.Fprintf(buffer, "metricName=%s,", pm.metricName)
	return buffer.String()
}

func mergeMetrics(pmb PrometheusMetricBatch) (result []*metricMaterial) {
	metricMap := make(map[string]*metricMaterial)
	for _, pm := range pmb {
		metricKey := getMetricKeyForMerging(pm)
		metricData := metricMap[metricKey]
		metricMap[metricKey] = mergePrometheusMetrics(metricData, pm)
	}
	for _, mm := range metricMap {
		result = append(result, mm)
	}
	return result
}

// return a metricMaterial merged with prometheusMetrics
func mergePrometheusMetrics(mm *metricMaterial, pm *PrometheusMetric) *metricMaterial {
	if mm == nil {
		// metricType is not propagated to metricMaterial intentionally.
		mm = &metricMaterial{tags: pm.tags, fields: map[string]interface{}{}, timeInMS: pm.timeInMS}
	}

	mm.fields[pm.metricName] = pm.metricValue
	return mm
}
