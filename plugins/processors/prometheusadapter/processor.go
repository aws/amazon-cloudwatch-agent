// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheusadapter

import (
	"context"
	"os"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
	"github.com/prometheus/common/model"
)

type prometheusAdapterProcessor struct {
	*Config
	logger *zap.Logger
}

func newPrometheusAdapterProcessor(config *Config, logger *zap.Logger) *prometheusAdapterProcessor {
	d := &prometheusAdapterProcessor{
		Config: config,
		logger: logger,
	}
	return d
}

func (d *prometheusAdapterProcessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		rma := rm.Resource().Attributes()
		sms := rm.ScopeMetrics()

		for j := 0; j < sms.Len(); j++ {
			metrics := sms.At(j).Metrics()

			// for backwards compatibility, we want to drop untyped metrics
			// untyped metrics are converted to Gauge by the receiver and the original type is stored in the metadata
			metrics.RemoveIf(func(m pmetric.Metric) bool {
				if typ, ok := m.Metadata().Get(prometheus.MetricMetadataTypeKey); ok {
					if typ.AsString() == string(model.MetricTypeUnknown) {
						d.logger.Debug("Drop untyped metric")
						return true
					}
				}
				return false
			})

			for k := 0; k < metrics.Len(); k++ {
				m := metrics.At(k)
				d.processMetric(m, rma)
			}

		}

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
	return md, nil
}

func (d *prometheusAdapterProcessor) processMetric(m pmetric.Metric, rma pcommon.Map) {
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		processNumberDataPointSlice(m.Gauge().DataPoints(), rma)
	case pmetric.MetricTypeSum:
		processNumberDataPointSlice(m.Sum().DataPoints(), rma)
	case pmetric.MetricTypeSummary:
		processSummaryDataPointSlice(m.Summary().DataPoints(), rma)
	case pmetric.MetricTypeHistogram:
		processHistogramDataPointSlice(m.Histogram().DataPoints(), rma)
	case pmetric.MetricTypeExponentialHistogram:
		processExponentialHistogramDataPointSlice(m.ExponentialHistogram().DataPoints(), rma)
	case pmetric.MetricTypeEmpty:
		d.logger.Debug("Ignore empty metric")
	default:
		d.logger.Debug("Ignore unknown metric type %s", zap.String("type", m.Type().String()))
	}
}

func processNumberDataPointSlice(dps pmetric.NumberDataPointSlice, rma pcommon.Map) {
	for i := 0; i < dps.Len(); i++ {
		updateDatapointAttributes(dps.At(i).Attributes(), rma)
	}
}

func processSummaryDataPointSlice(dps pmetric.SummaryDataPointSlice, rma pcommon.Map) {
	for i := 0; i < dps.Len(); i++ {
		updateDatapointAttributes(dps.At(i).Attributes(), rma)
	}
}

func processHistogramDataPointSlice(dps pmetric.HistogramDataPointSlice, rma pcommon.Map) {
	for i := 0; i < dps.Len(); i++ {
		updateDatapointAttributes(dps.At(i).Attributes(), rma)
	}
}

func processExponentialHistogramDataPointSlice(dps pmetric.ExponentialHistogramDataPointSlice, rma pcommon.Map) {
	for i := 0; i < dps.Len(); i++ {
		updateDatapointAttributes(dps.At(i).Attributes(), rma)
	}
}

func updateDatapointAttributes(attr pcommon.Map, rma pcommon.Map) {
	// add new attributes
	attr.PutStr("receiver", "prometheus")
	hostname, err := os.Hostname()
	if err == nil {
		attr.PutStr("host", hostname)
	}

	// relabel
	if serviceName, ok := rma.Get("service.name"); ok {
		attr.PutStr("job", serviceName.AsString())
	}
	if serviceInstanceId, ok := rma.Get("service.instance.id"); ok {
		attr.PutStr("instance", serviceInstanceId.AsString())
	}
}
