// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheusadapter

import (
	"context"
	"os"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/prometheusadapter/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
	"github.com/prometheus/common/model"
)

type prometheusAdapterProcessor struct {
	*Config
	logger          *zap.Logger
	deltaCalculator *internal.DeltaCalculator
}

func newPrometheusAdapterProcessor(config *Config, logger *zap.Logger) *prometheusAdapterProcessor {
	d := &prometheusAdapterProcessor{
		Config:          config,
		logger:          logger,
		deltaCalculator: internal.NewDeltaCalculator(),
	}
	return d
}

func (d *prometheusAdapterProcessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {

	d.preprocessFilter(md)

	md.ResourceMetrics().RemoveIf(func(rm pmetric.ResourceMetrics) bool {
		rma := rm.Resource().Attributes()
		sms := rm.ScopeMetrics()
		sms.RemoveIf(func(sm pmetric.ScopeMetrics) bool {
			metrics := sm.Metrics()
			metrics.RemoveIf(func(m pmetric.Metric) bool {
				return d.processMetric(m, rma)
			})
			return metrics.Len() == 0
		})
		return sms.Len() == 0
	})

	d.postprocessFilter(md)

	return md, nil
}

func (d *prometheusAdapterProcessor) preprocessFilter(md pmetric.Metrics) {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			metrics := sm.Metrics()

			const (
				keep = false
				drop = true
			)

			metrics.RemoveIf(func(m pmetric.Metric) bool {

				// for backwards compatibility with legacy Telegraf receiver, we want to drop some extraneous metrics
				extraneousMetrics := map[string]struct{}{
					"scrape_duration_seconds":               {},
					"scrape_samples_post_metric_relabeling": {},
					"scrape_samples_scraped":                {},
					"scrape_series_added":                   {},
					"up":                                    {},
				}
				_, ok := extraneousMetrics[m.Name()]
				if ok {
					return drop
				}

				// for backwards compatibility with legacy Telegraf receiver, we want to drop untyped metrics
				// untyped metrics are converted to Gauge by the receiver and the original type is stored in the metadata
				if typ, ok := m.Metadata().Get(prometheus.MetricMetadataTypeKey); ok && typ.AsString() == string(model.MetricTypeUnknown) {
					d.logger.Debug("Drop untyped metric")
					return drop
				}

				return keep
			})
		}
	}
}

func (d *prometheusAdapterProcessor) postprocessFilter(md pmetric.Metrics) {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		rma := rm.Resource().Attributes()

		// Remove extraneous resource attributes
		// This must be done after processing metrics so that the resource attributes can be moved to datapoint attributes
		rma.RemoveIf(func(key string, value pcommon.Value) bool {
			extraneousAttributes := map[string]struct{}{
				"http.scheme":         {},
				"net.host.port":       {},
				"net.host.name":       {},
				"server.port":         {},
				"server.address":      {},
				"service.instance.id": {},
				"service.name":        {},
				"url.scheme":          {},
			}
			_, ok := extraneousAttributes[key]
			return ok
		})
	}
}

func (d *prometheusAdapterProcessor) processMetric(m pmetric.Metric, rma pcommon.Map) bool {
	typ := m.Type()
	switch typ {
	case pmetric.MetricTypeGauge:
		d.processNumberDataPointSlice(m.Gauge().DataPoints(), m.Metadata(), typ, rma)
	case pmetric.MetricTypeSum:
		dps := m.Sum().DataPoints()
		d.processNumberDataPointSlice(dps, m.Metadata(), typ, rma)
		d.deltaCalculator.Calculate(m)
		return dps.Len() == 0
	case pmetric.MetricTypeSummary:
		dps := m.Summary().DataPoints()
		d.processSummaryDataPointSlice(dps, m.Metadata(), typ, rma)
		d.deltaCalculator.Calculate(m)
		return dps.Len() == 0
	case pmetric.MetricTypeHistogram:
		d.processHistogramDataPointSlice(m.Histogram().DataPoints(), m.Metadata(), typ, rma)
	case pmetric.MetricTypeExponentialHistogram:
		d.processExponentialHistogramDataPointSlice(m.ExponentialHistogram().DataPoints(), m.Metadata(), typ, rma)
	case pmetric.MetricTypeEmpty:
		d.logger.Debug("Ignore empty metric")
	default:
		d.logger.Debug("Ignore unknown metric type %s", zap.Int32("type", int32(typ)), zap.String("type_str", typ.String()))
	}

	return false
}

func (d *prometheusAdapterProcessor) processNumberDataPointSlice(dps pmetric.NumberDataPointSlice, metadata pcommon.Map, typ pmetric.MetricType, rma pcommon.Map) {
	for i := 0; i < dps.Len(); i++ {
		updateDatapointAttributes(dps.At(i).Attributes(), typ, rma)
	}
}

func (d *prometheusAdapterProcessor) processSummaryDataPointSlice(dps pmetric.SummaryDataPointSlice, metadata pcommon.Map, typ pmetric.MetricType, rma pcommon.Map) {
	for i := 0; i < dps.Len(); i++ {
		updateDatapointAttributes(dps.At(i).Attributes(), typ, rma)
	}
}

func (d *prometheusAdapterProcessor) processHistogramDataPointSlice(dps pmetric.HistogramDataPointSlice, metadata pcommon.Map, typ pmetric.MetricType, rma pcommon.Map) {
	for i := 0; i < dps.Len(); i++ {
		updateDatapointAttributes(dps.At(i).Attributes(), typ, rma)
	}
}

func (d *prometheusAdapterProcessor) processExponentialHistogramDataPointSlice(dps pmetric.ExponentialHistogramDataPointSlice, metadata pcommon.Map, typ pmetric.MetricType, rma pcommon.Map) {
	for i := 0; i < dps.Len(); i++ {
		updateDatapointAttributes(dps.At(i).Attributes(), typ, rma)
	}
}

// updateDatapointAttributes modifies the data point attributes to mimic how the original telegraf-based prometheus
// receiver formatted the datapoints' attributes. This is purely for maintaining backwards compatibility with the legacy
// receiver's behavior
func updateDatapointAttributes(attr pcommon.Map, typ pmetric.MetricType, rma pcommon.Map) {
	hostname, err := os.Hostname()
	if err == nil {
		attr.PutStr("host", hostname)
	}

	if serviceName, ok := rma.Get("service.name"); ok {
		attr.PutStr("job", serviceName.AsString())
	}
	if serviceInstanceId, ok := rma.Get("service.instance.id"); ok {
		attr.PutStr("instance", serviceInstanceId.AsString())
	}

	// OTel prometheus receiver labels counter types as "sum", but they need to be labeled as "counter" to maintain
	// backwards compatibility
	promMetricType := strings.ToLower(typ.String())
	if typ == pmetric.MetricTypeSum {
		promMetricType = string(model.MetricTypeCounter)
	}
	attr.PutStr("prom_metric_type", promMetricType)
}
