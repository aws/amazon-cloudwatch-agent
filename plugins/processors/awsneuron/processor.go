// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsneuron

import (
	"context"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type neuronProcessor struct {
	config *Config
	logger *zap.Logger
}

func newNeuronProcessor(config *Config, logger *zap.Logger) *neuronProcessor {
	return &neuronProcessor{
		config: config,
		logger: logger,
	}
}

func (p *neuronProcessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rs := rms.At(i)
		ilms := rs.ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ils := ilms.At(j)
			metrics := ils.Metrics()

			neuronHardwareInfo, found := findNeuronHardwareInfo(metrics)
			if found {
				coresPerDevice, foundCPD := getNeuronCoresPerDevice(neuronHardwareInfo)
				if !foundCPD {
					coresPerDevice = DefaultNeuronCorePerDevice
				}
				addNeuronDeviceToExistingMetrics(metrics, coresPerDevice)
				addEmptyMetrics(neuronHardwareInfo, metrics)
			}
			// Scale neuroncore_utilization_ratio from 0.0–1.0 ratio to 0–100 percent
			// to match the old Container Insights V1 behavior where the metric was
			// reported as a percentage (e.g., 10.0 for 10% utilization).
			scaleUtilizationToPercent(metrics)
		}
	}
	return md, nil
}

// addNeuronDeviceToExistingMetrics iterates over all metrics and adds a NeuronDevice
// datapoint attribute to any datapoint that has a neuroncore attribute but no NeuronDevice.
// The device index is derived as floor(core_index / cores_per_device).
func addNeuronDeviceToExistingMetrics(metrics pmetric.MetricSlice, coresPerDevice int) {
	for i := 0; i < metrics.Len(); i++ {
		m := metrics.At(i)
		var dps pmetric.NumberDataPointSlice
		switch m.Type() {
		case pmetric.MetricTypeGauge:
			dps = m.Gauge().DataPoints()
		case pmetric.MetricTypeSum:
			dps = m.Sum().DataPoints()
		default:
			continue
		}
		for j := 0; j < dps.Len(); j++ {
			dp := dps.At(j)
			if _, already := dp.Attributes().Get(NeuronDeviceAttributeKey); already {
				continue
			}
			coreVal, hasCoreAttr := dp.Attributes().Get(NeuronCoreAttributeKey)
			if !hasCoreAttr {
				continue
			}
			coreIndex, err := strconv.Atoi(coreVal.AsString())
			if err != nil {
				continue
			}
			dp.Attributes().PutStr(NeuronDeviceAttributeKey, strconv.Itoa(coreIndex/coresPerDevice))
		}
	}
}

// scaleUtilizationToPercent converts neuroncore_utilization_ratio values from
// 0.0–1.0 ratio to 0–100 percent to match old Container Insights V1 behavior.
func scaleUtilizationToPercent(metrics pmetric.MetricSlice) {
	for i := 0; i < metrics.Len(); i++ {
		m := metrics.At(i)
		if m.Name() != NeuronCoreUtilization {
			continue
		}
		var dps pmetric.NumberDataPointSlice
		switch m.Type() {
		case pmetric.MetricTypeGauge:
			dps = m.Gauge().DataPoints()
		case pmetric.MetricTypeSum:
			dps = m.Sum().DataPoints()
		default:
			continue
		}
		for j := 0; j < dps.Len(); j++ {
			dp := dps.At(j)
			dp.SetDoubleValue(dp.DoubleValue() * 100)
		}
	}
}

// findNeuronHardwareInfo scans the metric slice for a metric named
// "neuron_hardware_info" or "neuron_hardware" (the Prometheus receiver strips
// the _info suffix from info-type metrics). Returns the metric and true if found.
func findNeuronHardwareInfo(metrics pmetric.MetricSlice) (pmetric.Metric, bool) {
	for k := 0; k < metrics.Len(); k++ {
		m := metrics.At(k)
		if m.Name() == NeuronHardwareInfoKey || m.Name() == NeuronHardwareKey {
			return m, true
		}
	}
	return pmetric.NewMetric(), false
}

// getNeuronDeviceCount extracts the neuron_device_count attribute from the
// first datapoint of the neuron_hardware_info/neuron_hardware metric.
// Handles both Gauge and Sum metric types since the Prometheus receiver may
// convert info metrics to non-monotonic Sums.
func getNeuronDeviceCount(hardwareInfo pmetric.Metric) (int, bool) {
	var datapoints pmetric.NumberDataPointSlice
	switch hardwareInfo.Type() {
	case pmetric.MetricTypeGauge:
		datapoints = hardwareInfo.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		datapoints = hardwareInfo.Sum().DataPoints()
	default:
		return -1, false
	}
	if datapoints.Len() > 0 {
		val, found := datapoints.At(0).Attributes().Get(NeuronDeviceCountAttributeKey)
		if found {
			count, _ := strconv.Atoi(val.AsString())
			return count, true
		}
	}
	return -1, false
}

// getNeuronCoresPerDevice extracts the neuroncore_per_device_count attribute from the
// first datapoint of the neuron_hardware_info/neuron_hardware metric.
// Handles both Gauge and Sum metric types since the Prometheus receiver may
// convert info metrics to non-monotonic Sums.
func getNeuronCoresPerDevice(hardwareInfo pmetric.Metric) (int, bool) {
	var datapoints pmetric.NumberDataPointSlice
	switch hardwareInfo.Type() {
	case pmetric.MetricTypeGauge:
		datapoints = hardwareInfo.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		datapoints = hardwareInfo.Sum().DataPoints()
	default:
		return -1, false
	}
	if datapoints.Len() > 0 {
		val, found := datapoints.At(0).Attributes().Get(NeuronCorePerDeviceKey)
		if found {
			count, _ := strconv.Atoi(val.AsString())
			return count, true
		}
	}
	return -1, false
}

// addEmptyMetrics checks which expected metrics are missing from the batch and
// synthesizes zero-valued datapoints for them.
func addEmptyMetrics(hardwareInfo pmetric.Metric, metrics pmetric.MetricSlice) {
	metricFoundMap := make(map[string]bool)
	for k := range attributeConfig {
		metricFoundMap[k] = false
	}

	for i := 0; i < metrics.Len(); i++ {
		m := metrics.At(i)
		if _, ok := metricFoundMap[m.Name()]; ok {
			metricFoundMap[m.Name()] = true
		}
	}

	for k, found := range metricFoundMap {
		if found {
			continue
		}
		if strings.Contains(k, "core") {
			populateCoreMetrics(metrics, k, hardwareInfo)
		} else {
			populateNonCoreMetrics(metrics, k, attributeConfig[k], hardwareInfo)
		}
	}
}

// getHardwareDataPoints returns the NumberDataPointSlice from the hardware info
// metric regardless of whether it's a Gauge or Sum type.
func getHardwareDataPoints(hardwareInfo pmetric.Metric) pmetric.NumberDataPointSlice {
	switch hardwareInfo.Type() {
	case pmetric.MetricTypeGauge:
		return hardwareInfo.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		return hardwareInfo.Sum().DataPoints()
	default:
		return pmetric.NewNumberDataPointSlice()
	}
}

// copyInstanceLabels copies instance labels from the neuron_hardware_info
// datapoint to the target datapoint attributes.
func copyInstanceLabels(source pmetric.NumberDataPoint, target pcommon.Map) {
	for _, key := range instanceLabelKeys {
		if val, ok := source.Attributes().Get(key); ok {
			target.PutStr(key, val.AsString())
		}
	}
}

// populateCoreMetrics creates per-core zero-valued datapoints for a missing core metric.
// It creates device_count * cores_per_device datapoints, each with:
//   - neuroncore=<core_index> (lowercase only, no NeuronCore/NeuronDevice)
//   - memory_location="None" for memory metrics only (not for neuroncore_utilization_ratio)
//   - instance labels copied from neuron_hardware_info
//   - NO runtime_tag (idle state has no runtime)
func populateCoreMetrics(metrics pmetric.MetricSlice, metricName string, hardwareInfo pmetric.Metric) {
	neuronCoresPerDevice, foundCoresPerDevice := getNeuronCoresPerDevice(hardwareInfo)
	neuronDeviceCount, foundDeviceCount := getNeuronDeviceCount(hardwareInfo)
	if !foundCoresPerDevice || !foundDeviceCount {
		return
	}

	hwDatapoints := getHardwareDataPoints(hardwareInfo)
	if hwDatapoints.Len() == 0 {
		return
	}

	hwDatapoint := hwDatapoints.At(0)
	isMemoryMetric := coreMemoryMetrics[metricName]

	now := pcommon.NewTimestampFromTime(time.Now())

	metricToAdd := pmetric.NewMetric()
	metricToAdd.SetEmptyGauge()
	metricToAdd.SetName(metricName)
	emptyDatapoints := metricToAdd.Gauge().DataPoints()
	for coreIndex := 0; coreIndex < neuronCoresPerDevice*neuronDeviceCount; coreIndex++ {
		datapoint := emptyDatapoints.AppendEmpty()
		datapoint.SetTimestamp(now)
		datapoint.SetDoubleValue(0)
		datapoint.Attributes().PutStr(NeuronCoreAttributeKey, strconv.Itoa(coreIndex))
		datapoint.Attributes().PutStr(NeuronDeviceAttributeKey, strconv.Itoa(coreIndex/neuronCoresPerDevice))
		if isMemoryMetric {
			datapoint.Attributes().PutStr(MemoryLocation, MemoryLocationNone)
		}
		copyInstanceLabels(hwDatapoint, datapoint.Attributes())
	}

	metricToAdd.CopyTo(metrics.AppendEmpty())
}

// populateNonCoreMetrics creates zero-valued datapoints for a missing non-core metric.
// It creates one datapoint per variant value (e.g., 5 error types, 6 status types).
// Counter metrics (execution_errors_total, execution_status_total) are created as
// Sum with IsMonotonic=true. Gauge metrics are created as Gauge.
// No runtime_tag is set (idle state has no runtime).
// Instance labels are copied from neuron_hardware_info to each datapoint.
func populateNonCoreMetrics(metrics pmetric.MetricSlice, metricName string, attributeKeys []string, hardwareInfo pmetric.Metric) {
	hwDatapoints := getHardwareDataPoints(hardwareInfo)
	if hwDatapoints.Len() == 0 {
		return
	}
	hwDatapoint := hwDatapoints.At(0)

	now := pcommon.NewTimestampFromTime(time.Now())

	metricToAdd := pmetric.NewMetric()
	metricToAdd.SetName(metricName)

	isCounter := counterMetrics[metricName]

	// Set the metric type once before adding datapoints.
	if isCounter {
		metricToAdd.SetEmptySum()
		metricToAdd.Sum().SetIsMonotonic(true)
	} else {
		metricToAdd.SetEmptyGauge()
	}

	// For each attribute key, iterate over all variant values and create a datapoint per variant.
	for _, attrKey := range attributeKeys {
		variants, ok := nonCoreVariants[attrKey]
		if !ok {
			continue
		}
		for _, variantValue := range variants {
			if isCounter {
				dp := metricToAdd.Sum().DataPoints().AppendEmpty()
				dp.SetTimestamp(now)
				dp.SetDoubleValue(0)
				dp.Attributes().PutStr(attrKey, variantValue)
				copyInstanceLabels(hwDatapoint, dp.Attributes())
			} else {
				dp := metricToAdd.Gauge().DataPoints().AppendEmpty()
				dp.SetTimestamp(now)
				dp.SetDoubleValue(0)
				dp.Attributes().PutStr(attrKey, variantValue)
				copyInstanceLabels(hwDatapoint, dp.Attributes())
			}
		}
	}

	metricToAdd.CopyTo(metrics.AppendEmpty())
}
