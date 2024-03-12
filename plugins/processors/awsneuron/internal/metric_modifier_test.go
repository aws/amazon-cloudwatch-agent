package internal

import (
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"testing"
	"time"
)

var staticAttributes = map[string]any{
	"dummyAttributeKey1": "dummyAttributeValue1",
	"dummyAttributeKey2": "dummyAttributeValue2",
	"dummyAttributeKey3": "dummyAttributeValue3",
	"dummyAttributeKey4": "dummyAttributeValue4",
	"dummyAttributeKey5": "dummyAttributeValue5",
}

type MetricDefinition struct {
	MetricType        pmetric.MetricType
	MetricValues      []float64
	SpecialAttributes [][]string
}

var metricNameToMetricLayout = map[string]MetricDefinition{
	"execution_latency_seconds":                       {MetricType: pmetric.MetricTypeGauge, MetricValues: []float64{}, SpecialAttributes: [][]string{{"percentile", "p0"}, {"percentile", "p1"}, {"percentile", "p100"}, {"percentile", "p25"}, {"percentile", "p50"}, {"percentile", "p75"}, {"percentile", "p99"}}},
	"neuron_hardware":                                 {MetricType: pmetric.MetricTypeSum, MetricValues: []float64{}, SpecialAttributes: [][]string{{"neuron_device_count", "16", "neuroncore_per_device_count", "2"}}},
	"execution_errors_total":                          {MetricType: pmetric.MetricTypeSum, MetricValues: []float64{}, SpecialAttributes: [][]string{{"error_type", "generic"}, {"error_type", "numerical"}, {"error_type", "transient"}, {"error_type", "model"}, {"error_type", "runtime"}, {"error_type", "hardware"}}},
	"execution_errors_created":                        {MetricType: pmetric.MetricTypeSum, MetricValues: []float64{}, SpecialAttributes: [][]string{{"error_type", "generic"}, {"error_type", "numerical"}, {"error_type", "transient"}, {"error_type", "model"}, {"error_type", "runtime"}, {"error_type", "hardware"}}},
	"execution_status_total":                          {MetricType: pmetric.MetricTypeSum, MetricValues: []float64{}, SpecialAttributes: [][]string{{"status_type", "completed"}, {"status_type", "completed_with_err"}, {"status_type", "completed_with_num_err"}, {"status_type", "timed_out"}, {"status_type", "incorrect_input"}, {"status_type", "failed_to_queue"}}},
	"execution_status_created":                        {MetricType: pmetric.MetricTypeSum, MetricValues: []float64{}, SpecialAttributes: [][]string{{"status_type", "completed"}, {"status_type", "completed_with_err"}, {"status_type", "completed_with_num_err"}, {"status_type", "timed_out"}, {"status_type", "incorrect_input"}, {"status_type", "failed_to_queue"}}},
	"neuron_runtime_memory_used_bytes":                {MetricType: pmetric.MetricTypeGauge, MetricValues: []float64{}, SpecialAttributes: [][]string{{"memory_location", "host"}, {"memory_location", "neuron_device"}}},
	"neuroncore_utilization_ratio":                    {MetricType: pmetric.MetricTypeGauge, MetricValues: []float64{}, SpecialAttributes: [][]string{{"NeuronCore", "0"}, {"NeuronCore", "1"}, {"NeuronCore", "2"}}},
	"system_vcpu_usage_ratio":                         {MetricType: pmetric.MetricTypeGauge, MetricValues: []float64{}, SpecialAttributes: [][]string{}},
	"non_neuron_metric":                               {MetricType: pmetric.MetricTypeGauge, MetricValues: []float64{}, SpecialAttributes: [][]string{}},
	"neuron_execution_errors":                         {MetricType: pmetric.MetricTypeSum, MetricValues: []float64{}, SpecialAttributes: [][]string{{"error_type", "generic"}, {"error_type", "numerical"}, {"error_type", "transient"}, {"error_type", "model"}, {"error_type", "runtime"}, {"error_type", "hardware"}}},
	"neuron_execution_status":                         {MetricType: pmetric.MetricTypeSum, MetricValues: []float64{}, SpecialAttributes: [][]string{{"status_type", "completed"}, {"status_type", "completed_with_err"}, {"status_type", "completed_with_num_err"}, {"status_type", "timed_out"}, {"status_type", "incorrect_input"}, {"status_type", "failed_to_queue"}}},
	"neuroncore_memory_usage_model_shared_scratchpad": {MetricType: pmetric.MetricTypeGauge, MetricValues: []float64{}, SpecialAttributes: [][]string{{"NeuronCore", "0", "memory_location", "None"}, {"NeuronCore", "1", "memory_location", "None"}, {"NeuronCore", "2", "memory_location", "None"}}},
	"neurondevice_runtime_memory_used_bytes":          {MetricType: pmetric.MetricTypeGauge, MetricValues: []float64{}, SpecialAttributes: [][]string{{"memory_location", "host"}, {"memory_location", "neuron_device"}}},
	"neuron_execution_latency_seconds":                {MetricType: pmetric.MetricTypeGauge, MetricValues: []float64{}, SpecialAttributes: [][]string{{"percentile", "p0"}, {"percentile", "p1"}, {"percentile", "p100"}, {"percentile", "p25"}, {"percentile", "p50"}, {"percentile", "p75"}, {"percentile", "p99"}}},
	"neurondevice_hw_ecc_events":                      {MetricType: pmetric.MetricTypeSum, MetricValues: []float64{}, SpecialAttributes: [][]string{{"neuron_device_index", "1", "event_type", "total_mem_ecc_corrected"}, {"neuron_device_index", "1", "event_type", "total_mem_ecc_uncorrected"}, {"neuron_device_index", "1", "event_type", "total_sram_ecc_corrected"}, {"neuron_device_index", "1", "event_type", "total_sram_ecc_uncorrected"}}},
}

func TestMetricModifierForExecutionLatencyMetric(t *testing.T) {
	metricModifier := setupMetricModifier()
	actual := metricModifier.ModifyMetric(createMetricForKey("neuron_execution_latency_seconds"))

	expectedMetrics := map[string]pmetric.Metric{
		"neuron_execution_latency_seconds": {},
	}
	assertModifiedMetric(t, actual, expectedMetrics)
}

func setupMetricModifier() *MetricModifier {
	logger, _ := zap.NewDevelopment()
	return &MetricModifier{logger: logger}
}

func createMetricForKey(key string) pmetric.Metric {
	staticTimestamp := pcommon.NewTimestampFromTime(time.Date(2023, time.March, 12, 11, 0, 0, 0, time.UTC))
	metricDefinition := metricNameToMetricLayout[key]

	metric := pmetric.NewMetric()
	metric.SetName(key)
	datapoints := pmetric.NumberDataPointSlice{}
	if metricDefinition.MetricType == pmetric.MetricTypeGauge {
		datapoints = metric.SetEmptyGauge().DataPoints()
	} else {
		datapoints = metric.SetEmptySum().DataPoints()
	}

	for i := 0; i < len(metricDefinition.MetricValues); i++ {
		datapoint := datapoints.AppendEmpty()
		datapoint.SetDoubleValue(metricDefinition.MetricValues[i])
		datapoint.SetTimestamp(staticTimestamp)
		datapoint.Attributes().FromRaw(staticAttributes)

		for j := 0; j < len(metricDefinition.SpecialAttributes[i])-1; j = j + 2 {
			datapoint.Attributes().PutStr(metricDefinition.SpecialAttributes[i][j], metricDefinition.SpecialAttributes[i][j+1])
		}
	}

	return metric
}

func assertModifiedMetric(t *testing.T, actualSlice pmetric.MetricSlice, expectedMetrics map[string]pmetric.Metric) {
	for i := 0; i < actualSlice.Len(); i++ {
		actualMetric := actualSlice.At(i)
		expectedMetric, exists := expectedMetrics[actualMetric.Name()]

		assert.True(t, exists)
		assert.Equal(t, expectedMetric.Name(), actualMetric.Name())
		assert.Equal(t, expectedMetric.Type(), actualMetric.Type())

		actualDatapoints := pmetric.NumberDataPointSlice{}
		expectedDatapoints := pmetric.NumberDataPointSlice{}
		if actualMetric.Type() == pmetric.MetricTypeGauge {
			actualDatapoints = actualMetric.Gauge().DataPoints()
			expectedDatapoints = expectedMetric.Gauge().DataPoints()
		} else {
			actualDatapoints = actualMetric.Sum().DataPoints()
			expectedDatapoints = expectedMetric.Sum().DataPoints()
		}

		assert.Equal(t, expectedDatapoints.Len(), actualDatapoints.Len())

		for j := 0; j < actualDatapoints.Len(); j++ {
			actualDatapoint := actualDatapoints.At(j)
			expectedDatapoint := expectedDatapoints.At(j)

			assert.Equal(t, expectedDatapoint.Attributes(), actualDatapoint.Attributes())
			assert.Equal(t, expectedDatapoint.ValueType(), actualDatapoint.ValueType())
			assert.Equal(t, expectedDatapoint.DoubleValue(), actualDatapoint.DoubleValue())
			assert.Equal(t, expectedDatapoint.Timestamp(), actualDatapoint.Timestamp())
		}
	}
}
