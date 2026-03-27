// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsneuron

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// ---------- helpers ----------

// buildHardwareInfoMetric creates a Gauge metric named "neuron_hardware_info"
// with topology attributes and instance labels on its single datapoint.
func buildHardwareInfoMetric(metrics pmetric.MetricSlice, deviceCount, coresPerDevice int) {
	m := metrics.AppendEmpty()
	m.SetName(NeuronHardwareInfoKey)
	gauge := m.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetDoubleValue(1)
	dp.Attributes().PutStr(NeuronDeviceCountAttributeKey, strconv.Itoa(deviceCount))
	dp.Attributes().PutStr(NeuronCorePerDeviceKey, strconv.Itoa(coresPerDevice))
	// Instance labels present on every real neuron metric.
	dp.Attributes().PutStr("availability_zone", "us-east-1a")
	dp.Attributes().PutStr("instance_id", "i-0123456789abcdef0")
	dp.Attributes().PutStr("instance_name", "my-instance")
	dp.Attributes().PutStr("instance_type", "trn1.2xlarge")
	dp.Attributes().PutStr("region", "us-east-1")
	dp.Attributes().PutStr("subnet_id", "subnet-abc123")
}

// addGaugeMetric appends a Gauge metric with a single datapoint (value 42).
func addGaugeMetric(metrics pmetric.MetricSlice, name string) {
	m := metrics.AppendEmpty()
	m.SetName(name)
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetDoubleValue(42)
}

// allExpectedMetricNames returns the 10 metric names the processor synthesises.
func allExpectedMetricNames() []string {
	return []string{
		NeuronCoreUtilization,
		NeuronCoreMemoryUtilizationConstants,
		NeuronCoreMemoryUtilizationModelCode,
		NeuronCoreMemoryUtilizationSharedScratchpad,
		NeuronCoreMemoryUtilizationRuntimeMemory,
		NeuronCoreMemoryUtilizationTensors,
		NeuronExecutionStatus,
		NeuronExecutionErrors,
		NeuronRuntimeMemoryUsage,
		NeuronExecutionLatency,
	}
}

// findMetricByName returns the first metric with the given name from the slice.
func findMetricByName(metrics pmetric.MetricSlice, name string) (pmetric.Metric, bool) {
	for i := 0; i < metrics.Len(); i++ {
		if metrics.At(i).Name() == name {
			return metrics.At(i), true
		}
	}
	return pmetric.NewMetric(), false
}

// datapointCount returns the number of datapoints regardless of metric type.
func datapointCount(m pmetric.Metric) int {
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		return m.Gauge().DataPoints().Len()
	case pmetric.MetricTypeSum:
		return m.Sum().DataPoints().Len()
	default:
		return 0
	}
}

// collectAttrValues collects all values of a given attribute key across datapoints.
func collectAttrValues(m pmetric.Metric, attrKey string) []string {
	var vals []string
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		dps := m.Gauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			if v, ok := dps.At(i).Attributes().Get(attrKey); ok {
				vals = append(vals, v.AsString())
			}
		}
	case pmetric.MetricTypeSum:
		dps := m.Sum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			if v, ok := dps.At(i).Attributes().Get(attrKey); ok {
				vals = append(vals, v.AsString())
			}
		}
	}
	return vals
}

// ---------- tests ----------

func TestEmptyInputPassthrough(t *testing.T) {
	p := newNeuronProcessor(&Config{}, zap.NewNop())
	md := pmetric.NewMetrics()

	result, err := p.processMetrics(context.Background(), md)

	assert.NoError(t, err)
	assert.Equal(t, 0, result.MetricCount())
	assert.Equal(t, 0, result.ResourceMetrics().Len())
}

func TestBatchWithoutHardwareInfoPassthrough(t *testing.T) {
	p := newNeuronProcessor(&Config{}, zap.NewNop())
	md := pmetric.NewMetrics()
	metrics := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
	addGaugeMetric(metrics, "some_random_metric")
	addGaugeMetric(metrics, "another_metric")

	result, err := p.processMetrics(context.Background(), md)

	assert.NoError(t, err)
	assert.Equal(t, 2, result.MetricCount())
	rm := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	assert.Equal(t, "some_random_metric", rm.At(0).Name())
	assert.Equal(t, "another_metric", rm.At(1).Name())
}

func TestFullIdleStateSynthesis(t *testing.T) {
	p := newNeuronProcessor(&Config{}, zap.NewNop())
	deviceCount := 1
	coresPerDevice := 2
	totalCores := deviceCount * coresPerDevice

	md := pmetric.NewMetrics()
	metrics := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
	buildHardwareInfoMetric(metrics, deviceCount, coresPerDevice)

	result, err := p.processMetrics(context.Background(), md)
	assert.NoError(t, err)

	rm := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	// 1 (neuron_hardware_info) + 10 synthesized = 11
	assert.Equal(t, 11, rm.Len())

	// Collect synthesized metrics by name.
	synthesized := make(map[string]pmetric.Metric)
	for i := 0; i < rm.Len(); i++ {
		m := rm.At(i)
		if m.Name() != NeuronHardwareInfoKey {
			synthesized[m.Name()] = m
		}
	}

	// All 10 expected metrics should be present.
	for _, name := range allExpectedMetricNames() {
		_, ok := synthesized[name]
		assert.True(t, ok, "expected metric %s to be synthesized", name)
	}

	// Per-core metrics: 2 datapoints each (1 device × 2 cores).
	coreMetrics := []string{
		NeuronCoreUtilization,
		NeuronCoreMemoryUtilizationConstants,
		NeuronCoreMemoryUtilizationModelCode,
		NeuronCoreMemoryUtilizationSharedScratchpad,
		NeuronCoreMemoryUtilizationRuntimeMemory,
		NeuronCoreMemoryUtilizationTensors,
	}
	for _, name := range coreMetrics {
		m := synthesized[name]
		assert.Equal(t, totalCores, datapointCount(m),
			"core metric %s should have %d datapoints", name, totalCores)
	}

	// Non-core metrics: specific datapoint counts.
	assert.Equal(t, 5, datapointCount(synthesized[NeuronExecutionErrors]),
		"execution_errors_total should have 5 datapoints")
	assert.Equal(t, 6, datapointCount(synthesized[NeuronExecutionStatus]),
		"execution_status_total should have 6 datapoints")
	assert.Equal(t, 7, datapointCount(synthesized[NeuronExecutionLatency]),
		"execution_latency_seconds should have 7 datapoints")
	assert.Equal(t, 2, datapointCount(synthesized[NeuronRuntimeMemoryUsage]),
		"neuron_runtime_memory_used_bytes should have 2 datapoints")

	// All values should be 0.
	for _, name := range allExpectedMetricNames() {
		m := synthesized[name]
		switch m.Type() {
		case pmetric.MetricTypeGauge:
			dps := m.Gauge().DataPoints()
			for i := 0; i < dps.Len(); i++ {
				assert.Equal(t, float64(0), dps.At(i).DoubleValue(),
					"metric %s dp %d should be 0", name, i)
			}
		case pmetric.MetricTypeSum:
			dps := m.Sum().DataPoints()
			for i := 0; i < dps.Len(); i++ {
				assert.Equal(t, float64(0), dps.At(i).DoubleValue(),
					"metric %s dp %d should be 0", name, i)
			}
		}
	}
}

func TestPartialMetricsPresentOnlyMissingSynthesized(t *testing.T) {
	p := newNeuronProcessor(&Config{}, zap.NewNop())

	md := pmetric.NewMetrics()
	metrics := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
	buildHardwareInfoMetric(metrics, 1, 2)

	// Pre-add two metrics that should NOT be re-synthesized.
	addGaugeMetric(metrics, NeuronExecutionStatus)
	addGaugeMetric(metrics, NeuronCoreUtilization)

	result, err := p.processMetrics(context.Background(), md)
	assert.NoError(t, err)

	rm := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()

	// Count occurrences of each metric name.
	nameCount := make(map[string]int)
	for i := 0; i < rm.Len(); i++ {
		nameCount[rm.At(i).Name()]++
	}

	// Pre-existing metrics should appear exactly once (not duplicated).
	assert.Equal(t, 1, nameCount[NeuronExecutionStatus])
	assert.Equal(t, 1, nameCount[NeuronCoreUtilization])

	// The remaining 8 metrics should each appear once (synthesized).
	remaining := []string{
		NeuronCoreMemoryUtilizationConstants,
		NeuronCoreMemoryUtilizationModelCode,
		NeuronCoreMemoryUtilizationSharedScratchpad,
		NeuronCoreMemoryUtilizationRuntimeMemory,
		NeuronCoreMemoryUtilizationTensors,
		NeuronExecutionErrors,
		NeuronRuntimeMemoryUsage,
		NeuronExecutionLatency,
	}
	for _, name := range remaining {
		assert.Equal(t, 1, nameCount[name], "missing metric %s should be synthesized once", name)
	}

	// Total: 1 hardware_info + 2 pre-existing + 8 synthesized = 11
	assert.Equal(t, 11, rm.Len())
}

func TestPerCoreAttributeCorrectness(t *testing.T) {
	testCases := []struct {
		name           string
		deviceCount    int
		coresPerDevice int
	}{
		{"1x1", 1, 1},
		{"1x2", 1, 2},
		{"2x2", 2, 2},
		{"4x2", 4, 2},
		{"2x4", 2, 4},
		{"16x2", 16, 2},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := newNeuronProcessor(&Config{}, zap.NewNop())
			totalCores := tc.deviceCount * tc.coresPerDevice

			md := pmetric.NewMetrics()
			metrics := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
			buildHardwareInfoMetric(metrics, tc.deviceCount, tc.coresPerDevice)

			result, err := p.processMetrics(context.Background(), md)
			assert.NoError(t, err)

			rm := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()

			// Check neuroncore_utilization_ratio (no memory_location).
			utilM, found := findMetricByName(rm, NeuronCoreUtilization)
			assert.True(t, found)
			dps := utilM.Gauge().DataPoints()
			assert.Equal(t, totalCores, dps.Len())
			for j := 0; j < dps.Len(); j++ {
				dp := dps.At(j)
				// neuroncore=<index> (lowercase)
				coreVal, ok := dp.Attributes().Get(NeuronCoreAttributeKey)
				assert.True(t, ok)
				assert.Equal(t, strconv.Itoa(j), coreVal.AsString())
				// No runtime_tag
				_, hasRT := dp.Attributes().Get(RuntimeTag)
				assert.False(t, hasRT, "idle-state should not have runtime_tag")
				// No uppercase NeuronCore
				_, hasUC := dp.Attributes().Get("NeuronCore")
				assert.False(t, hasUC)
				// NeuronDevice should be set to floor(coreIndex / coresPerDevice)
				ndVal, hasND := dp.Attributes().Get(NeuronDeviceAttributeKey)
				assert.True(t, hasND, "per-core datapoint should have NeuronDevice")
				assert.Equal(t, strconv.Itoa(j/tc.coresPerDevice), ndVal.AsString(),
					"NeuronDevice should be floor(coreIndex / coresPerDevice)")
				// neuroncore_utilization_ratio must NOT have memory_location
				_, hasML := dp.Attributes().Get(MemoryLocation)
				assert.False(t, hasML, "utilization should not have memory_location")
			}

			// Check a memory metric (should have memory_location="None").
			memM, found := findMetricByName(rm, NeuronCoreMemoryUtilizationConstants)
			assert.True(t, found)
			memDps := memM.Gauge().DataPoints()
			assert.Equal(t, totalCores, memDps.Len())
			for j := 0; j < memDps.Len(); j++ {
				dp := memDps.At(j)
				coreVal, ok := dp.Attributes().Get(NeuronCoreAttributeKey)
				assert.True(t, ok)
				assert.Equal(t, strconv.Itoa(j), coreVal.AsString())
				mlVal, ok := dp.Attributes().Get(MemoryLocation)
				assert.True(t, ok, "memory metric should have memory_location")
				assert.Equal(t, MemoryLocationNone, mlVal.AsString())
				_, hasRT := dp.Attributes().Get(RuntimeTag)
				assert.False(t, hasRT)
			}
		})
	}
}

func TestNonCoreMetricVariantCompleteness(t *testing.T) {
	p := newNeuronProcessor(&Config{}, zap.NewNop())
	md := pmetric.NewMetrics()
	metrics := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
	buildHardwareInfoMetric(metrics, 1, 2)

	result, err := p.processMetrics(context.Background(), md)
	assert.NoError(t, err)

	rm := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()

	// execution_errors_total: 5 error_type variants
	errM, _ := findMetricByName(rm, NeuronExecutionErrors)
	errVals := collectAttrValues(errM, ErrorType)
	assert.ElementsMatch(t, []string{"numerical", "transient", "model", "runtime", "hardware"}, errVals)

	// execution_status_total: 6 status_type variants
	statusM, _ := findMetricByName(rm, NeuronExecutionStatus)
	statusVals := collectAttrValues(statusM, StatusType)
	assert.ElementsMatch(t, []string{
		"completed", "completed_with_err", "completed_with_num_err",
		"timed_out", "incorrect_input", "failed_to_queue",
	}, statusVals)

	// execution_latency_seconds: 7 percentile variants
	latM, _ := findMetricByName(rm, NeuronExecutionLatency)
	latVals := collectAttrValues(latM, Percentile)
	assert.ElementsMatch(t, []string{"p0", "p1", "p100", "p25", "p50", "p75", "p99"}, latVals)

	// neuron_runtime_memory_used_bytes: 2 memory_location variants
	memM, _ := findMetricByName(rm, NeuronRuntimeMemoryUsage)
	memVals := collectAttrValues(memM, MemoryLocation)
	assert.ElementsMatch(t, []string{"host", "neuron_device"}, memVals)
}

func TestCorrectMetricTypes(t *testing.T) {
	p := newNeuronProcessor(&Config{}, zap.NewNop())
	md := pmetric.NewMetrics()
	metrics := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
	buildHardwareInfoMetric(metrics, 1, 2)

	result, err := p.processMetrics(context.Background(), md)
	assert.NoError(t, err)

	rm := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()

	// execution_errors_total and execution_status_total should be Sum with IsMonotonic=true.
	for _, name := range []string{NeuronExecutionErrors, NeuronExecutionStatus} {
		m, found := findMetricByName(rm, name)
		assert.True(t, found, "%s should exist", name)
		assert.Equal(t, pmetric.MetricTypeSum, m.Type(), "%s should be Sum", name)
		assert.True(t, m.Sum().IsMonotonic(), "%s should be monotonic", name)
	}

	// All other synthesized metrics should be Gauge.
	gaugeMetrics := []string{
		NeuronCoreUtilization,
		NeuronCoreMemoryUtilizationConstants,
		NeuronCoreMemoryUtilizationModelCode,
		NeuronCoreMemoryUtilizationSharedScratchpad,
		NeuronCoreMemoryUtilizationRuntimeMemory,
		NeuronCoreMemoryUtilizationTensors,
		NeuronRuntimeMemoryUsage,
		NeuronExecutionLatency,
	}
	for _, name := range gaugeMetrics {
		m, found := findMetricByName(rm, name)
		assert.True(t, found, "%s should exist", name)
		assert.Equal(t, pmetric.MetricTypeGauge, m.Type(), "%s should be Gauge", name)
	}
}

func TestInstanceLabelPropagation(t *testing.T) {
	p := newNeuronProcessor(&Config{}, zap.NewNop())
	md := pmetric.NewMetrics()
	metrics := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
	buildHardwareInfoMetric(metrics, 1, 2)

	result, err := p.processMetrics(context.Background(), md)
	assert.NoError(t, err)

	rm := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()

	expectedLabels := map[string]string{
		"availability_zone": "us-east-1a",
		"instance_id":       "i-0123456789abcdef0",
		"instance_name":     "my-instance",
		"instance_type":     "trn1.2xlarge",
		"region":            "us-east-1",
		"subnet_id":         "subnet-abc123",
	}

	for _, name := range allExpectedMetricNames() {
		m, found := findMetricByName(rm, name)
		assert.True(t, found, "%s should exist", name)

		switch m.Type() {
		case pmetric.MetricTypeGauge:
			dps := m.Gauge().DataPoints()
			for i := 0; i < dps.Len(); i++ {
				for k, v := range expectedLabels {
					got, ok := dps.At(i).Attributes().Get(k)
					assert.True(t, ok, "%s dp %d should have %s", name, i, k)
					assert.Equal(t, v, got.AsString(), "%s dp %d %s", name, i, k)
				}
			}
		case pmetric.MetricTypeSum:
			dps := m.Sum().DataPoints()
			for i := 0; i < dps.Len(); i++ {
				for k, v := range expectedLabels {
					got, ok := dps.At(i).Attributes().Get(k)
					assert.True(t, ok, "%s dp %d should have %s", name, i, k)
					assert.Equal(t, v, got.AsString(), "%s dp %d %s", name, i, k)
				}
			}
		}
	}
}

func TestMissingTopologyAttributesGracefulFallback(t *testing.T) {
	p := newNeuronProcessor(&Config{}, zap.NewNop())

	t.Run("missing neuron_device_count", func(t *testing.T) {
		md := pmetric.NewMetrics()
		metrics := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
		m := metrics.AppendEmpty()
		m.SetName(NeuronHardwareInfoKey)
		gauge := m.SetEmptyGauge()
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetDoubleValue(1)
		// Only set cores_per_device, omit device_count.
		dp.Attributes().PutStr(NeuronCorePerDeviceKey, "2")

		result, err := p.processMetrics(context.Background(), md)
		assert.NoError(t, err)
		// neuron_hardware_info is present but topology is incomplete.
		// Non-core metrics may still be synthesized, but core metrics should not
		// (they need both device_count and cores_per_device).
		rm := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
		for i := 0; i < rm.Len(); i++ {
			name := rm.At(i).Name()
			if coreMemoryMetrics[name] || name == NeuronCoreUtilization {
				// Core metrics should have 0 datapoints (not synthesized).
				assert.Equal(t, 0, datapointCount(rm.At(i)),
					"core metric %s should not be synthesized without device_count", name)
			}
		}
	})

	t.Run("missing neuroncore_per_device_count skips processing", func(t *testing.T) {
		md := pmetric.NewMetrics()
		metrics := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
		m := metrics.AppendEmpty()
		m.SetName(NeuronHardwareInfoKey)
		gauge := m.SetEmptyGauge()
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetDoubleValue(1)
		// Only set device_count, omit cores_per_device.
		dp.Attributes().PutStr(NeuronDeviceCountAttributeKey, "1")

		result, err := p.processMetrics(context.Background(), md)
		assert.NoError(t, err)
		// When cores_per_device is missing, processing is skipped entirely.
		rm := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
		assert.Equal(t, 1, rm.Len(), "only neuron_hardware_info should remain")
		assert.Equal(t, NeuronHardwareInfoKey, rm.At(0).Name())
	})
}

// buildHardwareInfoMetricAsSum creates the hardware info metric as a non-monotonic Sum,
// which is what the Prometheus receiver actually produces (it strips the _info suffix
// and converts info metrics to Sum type).
func buildHardwareInfoMetricAsSum(metrics pmetric.MetricSlice, deviceCount, coresPerDevice int) {
	m := metrics.AppendEmpty()
	m.SetName(NeuronHardwareKey)
	sum := m.SetEmptySum()
	sum.SetIsMonotonic(false)
	dp := sum.DataPoints().AppendEmpty()
	dp.SetDoubleValue(1)
	dp.Attributes().PutStr(NeuronDeviceCountAttributeKey, strconv.Itoa(deviceCount))
	dp.Attributes().PutStr(NeuronCorePerDeviceKey, strconv.Itoa(coresPerDevice))
	dp.Attributes().PutStr("availability_zone", "us-east-1a")
	dp.Attributes().PutStr("instance_id", "i-0123456789abcdef0")
	dp.Attributes().PutStr("instance_name", "my-instance")
	dp.Attributes().PutStr("instance_type", "trn1.2xlarge")
	dp.Attributes().PutStr("region", "us-east-1")
	dp.Attributes().PutStr("subnet_id", "subnet-abc123")
}

// TestSumTypeHardwareInfoSynthesis verifies that the processor correctly handles
// the hardware info metric when it arrives as a Sum (non-monotonic) rather than
// a Gauge. The Prometheus receiver strips the _info suffix from info-type metrics
// and converts them to Sum type, so the metric arrives as "neuron_hardware" with
// type Sum instead of "neuron_hardware_info" with type Gauge.
func TestSumTypeHardwareInfoSynthesis(t *testing.T) {
	p := newNeuronProcessor(&Config{}, zap.NewNop())

	md := pmetric.NewMetrics()
	metrics := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
	buildHardwareInfoMetricAsSum(metrics, 1, 2)

	result, err := p.processMetrics(context.Background(), md)
	assert.NoError(t, err)

	rm := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()

	// All expected metrics should be synthesized.
	for _, name := range allExpectedMetricNames() {
		_, found := findMetricByName(rm, name)
		assert.True(t, found, "expected metric %s to be synthesized from Sum-type hardware info", name)
	}

	// Verify per-core metrics have neuroncore and neurondevice attributes.
	coreUtil, found := findMetricByName(rm, NeuronCoreUtilization)
	assert.True(t, found)
	assert.Equal(t, 2, datapointCount(coreUtil)) // 1 device * 2 cores
	coreVals := collectAttrValues(coreUtil, NeuronCoreAttributeKey)
	assert.Contains(t, coreVals, "0")
	assert.Contains(t, coreVals, "1")
	deviceVals := collectAttrValues(coreUtil, NeuronDeviceAttributeKey)
	assert.Contains(t, deviceVals, "0") // both cores on device 0
}

// TestUtilizationRatioScaledToPercent verifies that neuroncore_utilization_ratio
// values are scaled from 0.0–1.0 ratio to 0–100 percent.
func TestUtilizationRatioScaledToPercent(t *testing.T) {
	p := newNeuronProcessor(&Config{}, zap.NewNop())

	md := pmetric.NewMetrics()
	metrics := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
	buildHardwareInfoMetric(metrics, 1, 2)

	// Add a real neuroncore_utilization_ratio metric with ratio values (0.0–1.0).
	utilMetric := metrics.AppendEmpty()
	utilMetric.SetName(NeuronCoreUtilization)
	dps := utilMetric.SetEmptyGauge().DataPoints()

	dp0 := dps.AppendEmpty()
	dp0.SetDoubleValue(0.1) // 10%
	dp0.Attributes().PutStr(NeuronCoreAttributeKey, "0")

	dp1 := dps.AppendEmpty()
	dp1.SetDoubleValue(0.856) // 85.6%
	dp1.Attributes().PutStr(NeuronCoreAttributeKey, "1")

	result, err := p.processMetrics(context.Background(), md)
	assert.NoError(t, err)

	rm := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	utilM, found := findMetricByName(rm, NeuronCoreUtilization)
	assert.True(t, found, "neuroncore_utilization_ratio should exist")

	resultDps := utilM.Gauge().DataPoints()
	// Collect values by core index.
	values := make(map[string]float64)
	for i := 0; i < resultDps.Len(); i++ {
		dp := resultDps.At(i)
		coreVal, _ := dp.Attributes().Get(NeuronCoreAttributeKey)
		values[coreVal.AsString()] = dp.DoubleValue()
	}

	assert.InDelta(t, 10.0, values["0"], 0.001, "core 0 should be scaled to ~10%%")
	assert.InDelta(t, 85.6, values["1"], 0.001, "core 1 should be scaled to ~85.6%%")
}
