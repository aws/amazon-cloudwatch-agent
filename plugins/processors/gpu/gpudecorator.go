// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package gpu

import (
	"context"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

const (
	gpuMetric = "_gpu_"
)

var metricDuplicateTypes = []string{
	containerinsightscommon.TypeContainer,
	containerinsightscommon.TypePod,
	containerinsightscommon.TypeNode,
}

var renameMapForDcgm = map[string]string{
	"DCGM_FI_DEV_GPU_UTIL":      containerinsightscommon.GpuUtilization,
	"DCGM_FI_DEV_MEM_COPY_UTIL": containerinsightscommon.GpuMemUtilization,
	"DCGM_FI_DEV_FB_USED":       containerinsightscommon.GpuMemUsed,
	"DCGM_FI_DEV_FB_TOTAL":      containerinsightscommon.GpuMemTotal,
	"DCGM_FI_DEV_GPU_TEMP":      containerinsightscommon.GpuTemperature,
	"DCGM_FI_DEV_POWER_USAGE":   containerinsightscommon.GpuPowerDraw,
}

type metricMutationRule struct {
	sources        []string
	target         string
	removeOriginal bool
}

type metricMutator interface {
	Process(ms pmetric.Metrics) error
}

type attributeMutator interface {
	Process(m pmetric.Metric, attrs pcommon.Map, removeOriginal bool) error
}

type decorator struct {
	*Config
	logger            *zap.Logger
	cancelFunc        context.CancelFunc
	shutdownC         chan bool
	started           bool
	attributeMutators []attributeMutator
	metricMutators    []metricMutator
}

func newDecorator(config *Config, logger *zap.Logger) *decorator {
	_, cancel := context.WithCancel(context.Background())
	d := &decorator{
		Config:     config,
		logger:     logger,
		cancelFunc: cancel,
	}
	return d
}

func (d *decorator) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	if !d.started {
		return pmetric.NewMetrics(), nil
	}

	for _, metricMutator := range d.metricMutators {
		// crate memory total
		metricMutator.Process(md)
	}

	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rs := rms.At(i)
		ilms := rs.ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ils := ilms.At(j)
			metrics := ils.Metrics()
			d.normalize(ctx, metrics)
			for k := 0; k < metrics.Len(); k++ {
				m := metrics.At(k)
				d.processMetricAttributes(ctx, m)
			}
		}
	}
	return md, nil
}

func (d *decorator) normalize(_ context.Context, metrics pmetric.MetricSlice) {
	// duplicate metrics for metric types by normalizing names
	orgLen := metrics.Len()
	for i := 0; i < orgLen; i++ {
		metric := metrics.At(i)
		if newName, ok := renameMapForDcgm[metric.Name()]; ok {
			for _, dt := range metricDuplicateTypes {
				newMetric := pmetric.NewMetric()
				metric.CopyTo(newMetric)
				newMetric.SetName(containerinsightscommon.MetricName(dt, newName))
				newMetric.MoveTo(metrics.AppendEmpty())
			}
		}
	}
}

func (d *decorator) processMetricAttributes(_ context.Context, m pmetric.Metric) {
	if !strings.Contains(m.Name(), gpuMetric) {
		return
	}

	switch m.Type() {
	case pmetric.MetricTypeGauge:
		dps := m.Gauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			for _, mutator := range d.attributeMutators {
				err := mutator.Process(m, dps.At(i).Attributes(), false)
				if err != nil {
					d.logger.Debug("failed to process attributes", zap.Error(err))
				}
			}
		}
	case pmetric.MetricTypeSum:
		dps := m.Sum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			for _, mutator := range d.attributeMutators {
				err := mutator.Process(m, dps.At(i).Attributes(), false)
				if err != nil {
					d.logger.Debug("failed to process attributes", zap.Error(err))
				}
			}
		}
	default:
		d.logger.Debug("Ignore unknown metric type", zap.String("type", m.Type().String()))
	}
}

func (d *decorator) Shutdown(context.Context) error {
	close(d.shutdownC)
	d.cancelFunc()
	return nil
}

func (d *decorator) Start(ctx context.Context, _ component.Host) error {
	d.shutdownC = make(chan bool)
	logTypeMutator := NewLogTypeAttribute(d.logger)
	d.attributeMutators = []attributeMutator{logTypeMutator}
	metricCombiner := NewMetricCombiner(d.logger, metricMutationRule{sources: []string{"DCGM_FI_DEV_FB_USED", "DCGM_FI_DEV_FB_FREE"}, target: "DCGM_FI_DEV_FB_TOTAL"})
	d.metricMutators = []metricMutator{metricCombiner}
	d.started = true
	return nil
}
