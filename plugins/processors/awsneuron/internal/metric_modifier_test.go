package internal

import (
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"maps"
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
var staticTimestamp = pcommon.NewTimestampFromTime(time.Date(2023, time.March, 12, 11, 0, 0, 0, time.UTC))

type MetricDefinition struct {
	MetricType        pmetric.MetricType
	MetricValues      []float64
	SpecialAttributes [][]string
}

var metricNameToMetricLayout = map[string]MetricDefinition{
	"non_neuron_metric":                               {MetricType: pmetric.MetricTypeGauge, MetricValues: []float64{1}, SpecialAttributes: [][]string{}},
	"neuron_execution_errors":                         {MetricType: pmetric.MetricTypeSum, MetricValues: []float64{1, 2, 3, 4, 5, 6}, SpecialAttributes: [][]string{{"error_type", "generic"}, {"error_type", "numerical"}, {"error_type", "transient"}, {"error_type", "model"}, {"error_type", "runtime"}, {"error_type", "hardware"}}},
	"neuron_execution_status":                         {MetricType: pmetric.MetricTypeSum, MetricValues: []float64{1, 2, 3, 4, 5, 6}, SpecialAttributes: [][]string{{"status_type", "completed"}, {"status_type", "completed_with_err"}, {"status_type", "completed_with_num_err"}, {"status_type", "timed_out"}, {"status_type", "incorrect_input"}, {"status_type", "failed_to_queue"}}},
	"neuroncore_memory_usage_model_shared_scratchpad": {MetricType: pmetric.MetricTypeGauge, MetricValues: []float64{1, 2, 3}, SpecialAttributes: [][]string{{"NeuronCore", "0", "memory_location", "None", "PodName", "DummyPod"}, {"NeuronCore", "1", "memory_location", "None", "PodName", "DummyPod"}, {"NeuronCore", "2", "memory_location", "None", "PodName", "DummyPod"}}},
	"neurondevice_runtime_memory_used_bytes":          {MetricType: pmetric.MetricTypeGauge, MetricValues: []float64{1, 2}, SpecialAttributes: [][]string{{"memory_location", "host"}, {"memory_location", "neuron_device"}}},
	"neuron_execution_latency_seconds":                {MetricType: pmetric.MetricTypeGauge, MetricValues: []float64{0, 0, 0, 0, 1, 0, 0}, SpecialAttributes: [][]string{{"percentile", "p0"}, {"percentile", "p1"}, {"percentile", "p100"}, {"percentile", "p25"}, {"percentile", "p50"}, {"percentile", "p75"}, {"percentile", "p99"}}},
	"neurondevice_hw_ecc_events_total":                {MetricType: pmetric.MetricTypeSum, MetricValues: []float64{1, 2, 3, 4}, SpecialAttributes: [][]string{{"neuron_device_index", "1", "event_type", "mem_ecc_corrected", "PodName", "DummyPod"}, {"neuron_device_index", "1", "event_type", "mem_ecc_uncorrected", "PodName", "DummyPod"}, {"neuron_device_index", "1", "event_type", "sram_ecc_corrected", "PodName", "DummyPod"}, {"neuron_device_index", "1", "event_type", "sram_ecc_uncorrected", "PodName", "DummyPod"}}},
}

func setupMetricModifier() *MetricModifier {
	logger, _ := zap.NewDevelopment()
	return &MetricModifier{logger: logger}
}
func TestMetricModifierForExecutionLatencyMetric(t *testing.T) {
	metricModifier := setupMetricModifier()
	actual := metricModifier.ModifyMetric(createActualMetricForKey("neuron_execution_latency_seconds"))

	expectedMetrics := map[string]pmetric.Metric{
		"node_neuron_execution_latency_seconds": createExpectedMetric("node_neuron_execution_latency_seconds", false, []map[string]string{{"Type": "NodeAwsNeuron"}}, []float64{1}, pmetric.MetricTypeSum),
	}

	assertModifiedMetric(t, actual, expectedMetrics)
}
func TestMetricModifierForExecutionErrorMetric(t *testing.T) {
	metricModifier := setupMetricModifier()
	actual := metricModifier.ModifyMetric(createActualMetricForKey("neuron_execution_errors"))

	expectedMetrics := map[string]pmetric.Metric{
		"node_neuron_execution_errors_generic":   createExpectedMetric("node_neuron_execution_errors_generic", true, []map[string]string{{"Type": "NodeAwsNeuron"}}, []float64{1}, pmetric.MetricTypeSum),
		"node_neuron_execution_errors_numerical": createExpectedMetric("node_neuron_execution_errors_numerical", true, []map[string]string{{"Type": "NodeAwsNeuron"}}, []float64{2}, pmetric.MetricTypeSum),
		"node_neuron_execution_errors_transient": createExpectedMetric("node_neuron_execution_errors_transient", true, []map[string]string{{"Type": "NodeAwsNeuron"}}, []float64{3}, pmetric.MetricTypeSum),
		"node_neuron_execution_errors_model":     createExpectedMetric("node_neuron_execution_errors_model", true, []map[string]string{{"Type": "NodeAwsNeuron"}}, []float64{4}, pmetric.MetricTypeSum),
		"node_neuron_execution_errors_runtime":   createExpectedMetric("node_neuron_execution_errors_runtime", true, []map[string]string{{"Type": "NodeAwsNeuron"}}, []float64{5}, pmetric.MetricTypeSum),
		"node_neuron_execution_errors_hardware":  createExpectedMetric("node_neuron_execution_errors_hardware", true, []map[string]string{{"Type": "NodeAwsNeuron"}}, []float64{6}, pmetric.MetricTypeSum),
		"node_neuron_execution_errors_total":     createExpectedMetric("node_neuron_execution_errors_total", true, []map[string]string{{"Type": "NodeAwsNeuron"}}, []float64{21}, pmetric.MetricTypeSum),
	}

	assertModifiedMetric(t, actual, expectedMetrics)
}

func TestMetricModifierForExecutionStatusMetric(t *testing.T) {
	metricModifier := setupMetricModifier()
	actual := metricModifier.ModifyMetric(createActualMetricForKey("neuron_execution_status"))

	expectedMap := maps.Clone(staticAttributes)
	expectedMap["Type"] = "NodeAwsNeuron"

	expectedMetrics := map[string]pmetric.Metric{
		"node_neuron_execution_status_completed":              createExpectedMetric("node_neuron_execution_status_completed", true, []map[string]string{{"Type": "NodeAwsNeuron"}}, []float64{1}, pmetric.MetricTypeSum),
		"node_neuron_execution_status_completed_with_err":     createExpectedMetric("node_neuron_execution_status_completed_with_err", true, []map[string]string{{"Type": "NodeAwsNeuron"}}, []float64{2}, pmetric.MetricTypeSum),
		"node_neuron_execution_status_completed_with_num_err": createExpectedMetric("node_neuron_execution_status_completed_with_num_err", true, []map[string]string{{"Type": "NodeAwsNeuron"}}, []float64{3}, pmetric.MetricTypeSum),
		"node_neuron_execution_status_timed_out":              createExpectedMetric("node_neuron_execution_status_timed_out", true, []map[string]string{{"Type": "NodeAwsNeuron"}}, []float64{4}, pmetric.MetricTypeSum),
		"node_neuron_execution_status_incorrect_input":        createExpectedMetric("node_neuron_execution_status_incorrect_input", true, []map[string]string{{"Type": "NodeAwsNeuron"}}, []float64{5}, pmetric.MetricTypeSum),
		"node_neuron_execution_status_failed_to_queue":        createExpectedMetric("node_neuron_execution_status_failed_to_queue", true, []map[string]string{{"Type": "NodeAwsNeuron"}}, []float64{6}, pmetric.MetricTypeSum),
		"node_neuron_execution_status_total":                  createExpectedMetric("node_neuron_execution_status_total", true, []map[string]string{{"Type": "NodeAwsNeuron"}}, []float64{21}, pmetric.MetricTypeSum),
	}

	assertModifiedMetric(t, actual, expectedMetrics)
}

func TestMetricModifierForNeuronCoreMemoryUsageMetric(t *testing.T) {
	metricModifier := setupMetricModifier()
	actual := metricModifier.ModifyMetric(createActualMetricForKey("neuroncore_memory_usage_model_shared_scratchpad"))

	expectedMetrics := map[string]pmetric.Metric{
		"node_neuroncore_memory_usage_model_shared_scratchpad":      createExpectedMetric("node_neuroncore_memory_usage_model_shared_scratchpad", false, []map[string]string{{"NeuronCore": "0", "Type": "NodeAwsNeuronCore", "PodName": "DummyPod"}, {"NeuronCore": "1", "Type": "NodeAwsNeuronCore", "PodName": "DummyPod"}, {"NeuronCore": "2", "Type": "NodeAwsNeuronCore", "PodName": "DummyPod"}}, []float64{1, 2, 3}, pmetric.MetricTypeSum),
		"pod_neuroncore_memory_usage_model_shared_scratchpad":       createExpectedMetric("pod_neuroncore_memory_usage_model_shared_scratchpad", false, []map[string]string{{"NeuronCore": "0", "Type": "PodAwsNeuronCore", "PodName": "DummyPod"}, {"NeuronCore": "1", "Type": "PodAwsNeuronCore", "PodName": "DummyPod"}, {"NeuronCore": "2", "Type": "PodAwsNeuronCore", "PodName": "DummyPod"}}, []float64{1, 2, 3}, pmetric.MetricTypeSum),
		"container_neuroncore_memory_usage_model_shared_scratchpad": createExpectedMetric("container_neuroncore_memory_usage_model_shared_scratchpad", false, []map[string]string{{"NeuronCore": "0", "Type": "ContainerAwsNeuronCore", "PodName": "DummyPod"}, {"NeuronCore": "1", "Type": "ContainerAwsNeuronCore", "PodName": "DummyPod"}, {"NeuronCore": "2", "Type": "ContainerAwsNeuronCore", "PodName": "DummyPod"}}, []float64{1, 2, 3}, pmetric.MetricTypeSum),
	}

	assertModifiedMetric(t, actual, expectedMetrics)
}

func TestMetricModifierForNeuronDeviceRuntimeMemoryUsageMetric(t *testing.T) {
	metricModifier := setupMetricModifier()
	actual := metricModifier.ModifyMetric(createActualMetricForKey("neurondevice_runtime_memory_used_bytes"))

	expectedMetrics := map[string]pmetric.Metric{
		"node_neurondevice_runtime_memory_used_bytes": createExpectedMetric("node_neurondevice_runtime_memory_used_bytes", false, []map[string]string{{"Type": "NodeAwsNeuron"}}, []float64{2}, pmetric.MetricTypeSum),
	}

	assertModifiedMetric(t, actual, expectedMetrics)
}

func TestMetricModifierForNeuronDeviceEccEventMetric(t *testing.T) {
	metricModifier := setupMetricModifier()
	actual := metricModifier.ModifyMetric(createActualMetricForKey("neurondevice_hw_ecc_events_total"))

	expectedMetrics := map[string]pmetric.Metric{
		"node_neurondevice_hw_ecc_events_total_mem_ecc_corrected":         createExpectedMetric("node_neurondevice_hw_ecc_events_total_mem_ecc_corrected", false, []map[string]string{{"neuron_device_index": "1", "PodName": "DummyPod", "Type": "NodeAwsNeuronDevice"}}, []float64{1}, pmetric.MetricTypeSum),
		"node_neurondevice_hw_ecc_events_total_mem_ecc_uncorrected":       createExpectedMetric("node_neurondevice_hw_ecc_events_total_mem_ecc_uncorrected", false, []map[string]string{{"neuron_device_index": "1", "PodName": "DummyPod", "Type": "NodeAwsNeuronDevice"}}, []float64{2}, pmetric.MetricTypeSum),
		"node_neurondevice_hw_ecc_events_total_sram_ecc_corrected":        createExpectedMetric("node_neurondevice_hw_ecc_events_total_sram_ecc_corrected", false, []map[string]string{{"neuron_device_index": "1", "PodName": "DummyPod", "Type": "NodeAwsNeuronDevice"}}, []float64{3}, pmetric.MetricTypeSum),
		"node_neurondevice_hw_ecc_events_total_sram_ecc_uncorrected":      createExpectedMetric("node_neurondevice_hw_ecc_events_total_sram_ecc_uncorrected", false, []map[string]string{{"neuron_device_index": "1", "PodName": "DummyPod", "Type": "NodeAwsNeuronDevice"}}, []float64{4}, pmetric.MetricTypeSum),
		"pod_neurondevice_hw_ecc_events_total_mem_ecc_corrected":          createExpectedMetric("pod_neurondevice_hw_ecc_events_total_mem_ecc_corrected", false, []map[string]string{{"neuron_device_index": "1", "PodName": "DummyPod", "Type": "PodAwsNeuronDevice"}}, []float64{1}, pmetric.MetricTypeSum),
		"pod_neurondevice_hw_ecc_events_total_mem_ecc_uncorrected":        createExpectedMetric("pod_neurondevice_hw_ecc_events_total_mem_ecc_uncorrected", false, []map[string]string{{"neuron_device_index": "1", "PodName": "DummyPod", "Type": "PodAwsNeuronDevice"}}, []float64{2}, pmetric.MetricTypeSum),
		"pod_neurondevice_hw_ecc_events_total_sram_ecc_corrected":         createExpectedMetric("pod_neurondevice_hw_ecc_events_total_sram_ecc_corrected", false, []map[string]string{{"neuron_device_index": "1", "PodName": "DummyPod", "Type": "PodAwsNeuronDevice"}}, []float64{3}, pmetric.MetricTypeSum),
		"pod_neurondevice_hw_ecc_events_total_sram_ecc_uncorrected":       createExpectedMetric("pod_neurondevice_hw_ecc_events_total_sram_ecc_uncorrected", false, []map[string]string{{"neuron_device_index": "1", "PodName": "DummyPod", "Type": "PodAwsNeuronDevice"}}, []float64{4}, pmetric.MetricTypeSum),
		"container_neurondevice_hw_ecc_events_total_mem_ecc_corrected":    createExpectedMetric("container_neurondevice_hw_ecc_events_total_mem_ecc_corrected", false, []map[string]string{{"neuron_device_index": "1", "PodName": "DummyPod", "Type": "ContainerAwsNeuronDevice"}}, []float64{1}, pmetric.MetricTypeSum),
		"container_neurondevice_hw_ecc_events_total_mem_ecc_uncorrected":  createExpectedMetric("container_neurondevice_hw_ecc_events_total_mem_ecc_uncorrected", false, []map[string]string{{"neuron_device_index": "1", "PodName": "DummyPod", "Type": "ContainerAwsNeuronDevice"}}, []float64{2}, pmetric.MetricTypeSum),
		"container_neurondevice_hw_ecc_events_total_sram_ecc_corrected":   createExpectedMetric("container_neurondevice_hw_ecc_events_total_sram_ecc_corrected", false, []map[string]string{{"neuron_device_index": "1", "PodName": "DummyPod", "Type": "ContainerAwsNeuronDevice"}}, []float64{3}, pmetric.MetricTypeSum),
		"container_neurondevice_hw_ecc_events_total_sram_ecc_uncorrected": createExpectedMetric("container_neurondevice_hw_ecc_events_total_sram_ecc_uncorrected", false, []map[string]string{{"neuron_device_index": "1", "PodName": "DummyPod", "Type": "ContainerAwsNeuronDevice"}}, []float64{4}, pmetric.MetricTypeSum),
	}

	assertModifiedMetric(t, actual, expectedMetrics)
}

func TestMetricModifierForNonNeuronMonitorMetric(t *testing.T) {
	metricModifier := setupMetricModifier()
	actual := metricModifier.ModifyMetric(createActualMetricForKey("non_neuron_metric"))

	expectedMetrics := map[string]pmetric.Metric{
		"non_neuron_metric": createExpectedMetric("non_neuron_metric", false, []map[string]string{{}}, []float64{1}, pmetric.MetricTypeGauge),
	}

	assertModifiedMetric(t, actual, expectedMetrics)
}

func createActualMetricForKey(key string) pmetric.Metric {
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

		if len(metricDefinition.SpecialAttributes) > 0 {
			for j := 0; j < len(metricDefinition.SpecialAttributes[i])-1; j = j + 2 {
				datapoint.Attributes().PutStr(metricDefinition.SpecialAttributes[i][j], metricDefinition.SpecialAttributes[i][j+1])
			}
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

			assert.Equal(t, expectedDatapoint.Attributes().Len(), actualDatapoint.Attributes().Len())
			for key, val := range actualDatapoint.Attributes().AsRaw() {
				expectedVal, _ := expectedDatapoint.Attributes().Get(key)
				assert.Equal(t, expectedVal.AsString(), val)
			}

			assert.Equal(t, expectedDatapoint.ValueType(), actualDatapoint.ValueType())
			assert.Equal(t, expectedDatapoint.DoubleValue(), actualDatapoint.DoubleValue())
			assert.Equal(t, expectedDatapoint.Timestamp(), actualDatapoint.Timestamp())
		}
	}
}

func createExpectedMetric(name string, isCumulative bool, attributes []map[string]string, values []float64, metricType pmetric.MetricType) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)

	datapoints := metric.SetEmptySum().DataPoints()
	if metricType == pmetric.MetricTypeGauge {
		datapoints = metric.SetEmptyGauge().DataPoints()
	}

	if isCumulative {
		metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	}

	for i := 0; i < len(values); i++ {
		datapoint := datapoints.AppendEmpty()
		datapoint.SetTimestamp(staticTimestamp)
		datapoint.SetDoubleValue(values[i])
		datapoint.Attributes().FromRaw(staticAttributes)

		for key, val := range attributes[i] {
			datapoint.Attributes().PutStr(key, val)
		}
	}

	return metric
}
