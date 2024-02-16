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

var renameMapForDcgm = map[string]string{
	"DCGM_FI_DEV_GPU_UTIL":        containerinsightscommon.GpuUtilization,
	"DCGM_FI_DEV_FB_USED_PERCENT": containerinsightscommon.GpuMemUtilization,
	"DCGM_FI_DEV_FB_USED":         containerinsightscommon.GpuMemUsed,
	"DCGM_FI_DEV_FB_TOTAL":        containerinsightscommon.GpuMemTotal,
	"DCGM_FI_DEV_GPU_TEMP":        containerinsightscommon.GpuTemperature,
	"DCGM_FI_DEV_POWER_USAGE":     containerinsightscommon.GpuPowerDraw,
	// "DCGM_FI_DEV_FAN_SPEED":       containerinsightscommon.GpuFanSpeed,
}

type gpuprocessor struct {
	*Config
	logger     *zap.Logger
	cancelFunc context.CancelFunc
	shutdownC  chan bool
	started    bool
}

func newGpuProcessor(config *Config, logger *zap.Logger) *gpuprocessor {
	_, cancel := context.WithCancel(context.Background())
	d := &gpuprocessor{
		Config:     config,
		logger:     logger,
		cancelFunc: cancel,
	}
	return d
}

func (d *gpuprocessor) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	if !d.started {
		return pmetric.NewMetrics(), nil
	}

	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rs := rms.At(i)
		ilms := rs.ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ils := ilms.At(j)
			metrics := ils.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				m := metrics.At(k)
				d.processMetricAttributes(ctx, m)
			}
		}
	}
	return md, nil
}

func (d *gpuprocessor) processMetricAttributes(_ context.Context, m pmetric.Metric) {
	// only decorate GPU metrics
	// another option is to separate GPU of its own pipeline to minimize extra processing of metrics
	if !strings.Contains(m.Name(), gpuMetric) {
		return
	}

	switch m.Type() {
	case pmetric.MetricTypeGauge:
		dps := m.Gauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			addDefaultAttributes(dps.At(i).Attributes())
		}
	case pmetric.MetricTypeSum:
		dps := m.Sum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			addDefaultAttributes(dps.At(i).Attributes())
		}
	default:
		d.logger.Debug("Ignore unknown metric type", zap.String("type", m.Type().String()))
	}
}

func addDefaultAttributes(attributes pcommon.Map) {
	for _, k := range defaultGpuLabels {
		if _, ok := attributes.Get(k); !ok {
			attributes.PutStr(k, "")
		}
	}
}

func (d *gpuprocessor) Shutdown(context.Context) error {
	close(d.shutdownC)
	d.cancelFunc()
	return nil
}

func (d *gpuprocessor) Start(ctx context.Context, _ component.Host) error {
	d.shutdownC = make(chan bool)
	d.started = true
	return nil
}
