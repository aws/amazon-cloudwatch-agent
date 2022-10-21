// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"log"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/metric/distribution"
	"github.com/aws/amazon-cloudwatch-agent/metric/distribution/seh1"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/metric"
	"github.com/stretchr/testify/assert"
)

var metricName = "metric1"
var wg sync.WaitGroup

func TestAggregator_NoAggregationKeyFound(t *testing.T) {
	metricChan, shutdownChan, aggregator := testPreparation()
	//no aggregation key found
	tags := map[string]string{"d1key": "d1value", "d2key": "d2value"}
	fields := map[string]interface{}{"value": 1}
	timestamp := time.Now()
	m := metric.New(metricName, tags, fields, timestamp)

	aggregator.AddMetric(m)
	select {
	case aggregatedMetric := <-metricChan:
		assert.False(t, aggregatedMetric.HasTag(aggregationIntervalTagKey))
		assert.Equal(t, m, aggregatedMetric)
	default:
		assert.Fail(t, "Got no metrics")
	}

	assertNoMetricsInChan(t, metricChan)
	close(shutdownChan)
	// Cleanup
	wg.Wait()
}

func TestAggregator_NotDurationType(t *testing.T) {
	metricChan, shutdownChan, aggregator := testPreparation()
	//aggregation key found, but no time.Duration type for the value
	tags := map[string]string{"d1key": "d1value", "d2key": "d2value", aggregationIntervalTagKey: "1"}
	fields := map[string]interface{}{"value": 1}
	timestamp := time.Now()
	m := metric.New(metricName, tags, fields, timestamp)

	aggregator.AddMetric(m)
	select {
	case aggregatedMetric := <-metricChan:
		assert.False(t, aggregatedMetric.HasTag(aggregationIntervalTagKey))
		assert.Equal(t, m, aggregatedMetric)
	default:
		assert.Fail(t, "Got no metrics")
	}

	assertNoMetricsInChan(t, metricChan)
	close(shutdownChan)
	// Cleanup
	wg.Wait()
}

func TestAggregator_ProperAggregationKey(t *testing.T) {
	metricChan, shutdownChan, aggregator := testPreparation()
	//normal proper aggregation key found
	aggregationInterval := 1 * time.Second
	tags := map[string]string{"d1key": "d1value", "d2key": "d2value", aggregationIntervalTagKey: aggregationInterval.String()}
	fields := map[string]interface{}{"value": 1}
	timestamp := time.Now()
	m := metric.New(metricName, tags, fields, timestamp)

	aggregator.AddMetric(m)
	assertNoMetricsInChan(t, metricChan)
	assertMetricContent(t, metricChan, aggregationInterval*2, m, expectedFieldContent{"value", 1, 1, 1, 1, "",
		[]float64{1.0488088481701516}, []float64{1}})

	assertNoMetricsInChan(t, metricChan)
	close(shutdownChan)
	// Cleanup
	wg.Wait()
}

func TestAggregator_MultipleAggregationPeriods(t *testing.T) {
	metricChan, shutdownChan, aggregator := testPreparation()
	//multiple metrics with multiple aggregation period, some aggregation period are the same, some are not.
	timestamp := time.Now()
	aggregationInterval := 1 * time.Second

	tags := map[string]string{"d1key": "d1value", "d2key": "d2value", aggregationIntervalTagKey: aggregationInterval.String()}
	fields := map[string]interface{}{"value": 1}
	m := metric.New(metricName, tags, fields, timestamp)
	aggregator.AddMetric(m)

	fields = map[string]interface{}{"value": 2}
	m = metric.New(metricName, tags, fields, timestamp)
	aggregator.AddMetric(m)

	fields = map[string]interface{}{"value": 3}
	m = metric.New(metricName, tags, fields, timestamp)
	aggregator.AddMetric(m)

	tags = map[string]string{"d1key": "d1value", "d2key": "d2value", aggregationIntervalTagKey: (2 * aggregationInterval).String()}
	fields = map[string]interface{}{"value": 4, "2nd value": 1}
	m = metric.New(metricName, tags, fields, timestamp)
	aggregator.AddMetric(m)

	fields = map[string]interface{}{"value": 5, "2nd value": 2}
	m = metric.New(metricName, tags, fields, timestamp)
	aggregator.AddMetric(m)

	assertNoMetricsInChan(t, metricChan)
	assertMetricContent(t, metricChan, aggregationInterval*3, m, expectedFieldContent{"value", 3, 1, 3, 6, "",
		[]float64{1.0488088481701516, 2.0438317370604793, 2.992374046230249}, []float64{1, 1, 1}})

	assertNoMetricsInChan(t, metricChan)

	assertMetricContent(t, metricChan, aggregationInterval*3, m, expectedFieldContent{"value", 5, 4, 2, 9, "",
		[]float64{3.9828498555324616, 4.819248325194279}, []float64{1, 1}},
		expectedFieldContent{"2nd value", 2, 1, 2, 3, "",
			[]float64{1.0488088481701516, 2.0438317370604793}, []float64{1, 1}})

	assertNoMetricsInChan(t, metricChan)
	close(shutdownChan)
	// Cleanup
	wg.Wait()
}

func TestAggregator_ShutdownBehavior(t *testing.T) {
	metricChan, shutdownChan, aggregator := testPreparation()
	// verify the remaining metrics can be read after shutdown
	// the metrics should be available immediately after the shutdown even before aggregation period
	aggregationInterval := 2 * time.Second
	tags := map[string]string{
		"d1key":                   "d1value",
		"d2key":                   "d2value",
		aggregationIntervalTagKey: aggregationInterval.String()}
	fields := map[string]interface{}{"value": 1}
	timestamp := time.Now()
	m := metric.New(metricName, tags, fields, timestamp)
	aggregator.AddMetric(m)
	// The Aggregator creates a new durationAggregator for each metric.
	// And there is a delay when each new durationAggregator begins.
	// So submit a metric and wait for the first aggregation to occur.
	assertMetricContent(t, metricChan, 3*aggregationInterval, m, expectedFieldContent{
		"value", 1, 1, 1, 1, "", []float64{1.0488088481701516}, []float64{1}})
	assertNoMetricsInChan(t, metricChan)
	// Now submit the same metric and it should be routed to the existing
	// durationAggregator without delay.
	timestamp = time.Now()
	m = metric.New(metricName, tags, fields, timestamp)
	aggregator.AddMetric(m)
	// Shutdown before the 2nd aggregationInterval completes.
	close(shutdownChan)
	wg.Wait()
	assertMetricContent(t, metricChan, 1*time.Second, m, expectedFieldContent{
		"value", 1, 1, 1, 1, "", []float64{1.0488088481701516}, []float64{1}})
	assertNoMetricsInChan(t, metricChan)
}

func TestDurationAggregator_aggregating(t *testing.T) {
	aggregationInterval := time.Second
	metricChan := make(chan telegraf.Metric, metricChanBufferSize)
	shutdownChan := make(chan struct{})
	durationAgg := &durationAggregator{
		aggregationDuration: aggregationInterval,
		metricChan:          metricChan,
		shutdownChan:        shutdownChan,
		wg:                  &wg,
		metricMap:           make(map[string]telegraf.Metric),
		aggregationChan:     make(chan telegraf.Metric, durationAggregationChanBufferSize),
	}
	go durationAgg.aggregating()

	timestamp := time.Now()

	tags := map[string]string{"d1key": "d1value", "d2key": "d2value", aggregationIntervalTagKey: aggregationInterval.String()}
	fields := map[string]interface{}{"value": 1}
	m := metric.New(metricName, tags, fields, timestamp)
	durationAgg.addMetric(m)
	m = metric.New(metricName, tags, fields, timestamp.Add(aggregationInterval))
	durationAgg.addMetric(m)

	fields = map[string]interface{}{"value": 2}
	m = metric.New(metricName, tags, fields, timestamp)
	durationAgg.addMetric(m)
	m = metric.New(metricName, tags, fields, timestamp.Add(aggregationInterval*2))
	durationAgg.addMetric(m)

	fields = map[string]interface{}{"value": 3}
	m = metric.New(metricName, tags, fields, timestamp)
	durationAgg.addMetric(m)
	m = metric.New(metricName, tags, fields, timestamp.Add(aggregationInterval*3))
	durationAgg.addMetric(m)

	//give some time to aggregation to do the work
	time.Sleep(2 * time.Second)

	close(shutdownChan)
	wg.Wait()
	assert.Empty(t, durationAgg.aggregationChan)
	assert.Empty(t, durationAgg.metricMap)
	assert.Equal(t, 4, len(durationAgg.metricChan))
}

type expectedFieldContent struct {
	fieldName                  string
	max, min, sampleCount, sum float64
	unit                       string
	expectedValues             []float64
	expectedCounts             []float64
}

func testPreparation() (chan telegraf.Metric, chan struct{}, Aggregator) {
	distribution.NewDistribution = seh1.NewSEH1Distribution
	metricChan := make(chan telegraf.Metric, metricChanBufferSize)
	shutdownChan := make(chan struct{})
	aggregator := NewAggregator(metricChan, shutdownChan, &wg)
	return metricChan, shutdownChan, aggregator
}

func assertMetricContent(t *testing.T, metricChan <-chan telegraf.Metric, metricMaxWait time.Duration, originalMetric telegraf.Metric, expectedFieldContent ...expectedFieldContent) {
	var aggregatedMetric telegraf.Metric

	log.Printf("Waiting for metric.")
	select {
	case aggregatedMetric = <-metricChan:
	case <-time.After(metricMaxWait):
		assert.FailNow(t, "We should've seen 1 metric by now")
	}

	log.Printf("Checking metric.")

	assert.False(t, aggregatedMetric.HasTag(aggregationIntervalTagKey))
	assert.Equal(t, "true", aggregatedMetric.Tags()[highResolutionTagKey])

	for _, fieldContent := range expectedFieldContent {
		dist, ok := aggregatedMetric.Fields()[fieldContent.fieldName].(distribution.Distribution)
		assert.True(t, ok)

		assert.Equal(t, fieldContent.max, dist.Maximum())
		assert.Equal(t, fieldContent.sampleCount, dist.SampleCount())
		assert.Equal(t, fieldContent.unit, dist.Unit())
		assert.Equal(t, fieldContent.min, dist.Minimum())
		assert.Equal(t, fieldContent.sum, dist.Sum())

		values, counts := dist.ValuesAndCounts()
		assert.Equal(t, len(fieldContent.expectedValues), len(values))
		assert.Equal(t, len(fieldContent.expectedCounts), len(counts))

		sort.Float64s(fieldContent.expectedValues)
		sort.Float64s(values)
		assert.Equal(t, fieldContent.expectedValues, values)

		var expectedCountInts []int
		for _, count := range fieldContent.expectedCounts {
			expectedCountInts = append(expectedCountInts, int(count))
		}
		sort.Ints(expectedCountInts)
		var countInts []int
		for _, count := range counts {
			countInts = append(countInts, int(count))
		}
		sort.Ints(countInts)
		assert.Equal(t, expectedCountInts, countInts)
	}

	assert.NotEqual(t, originalMetric, aggregatedMetric, "The aggregatedMetric should not exactly equal to m since the field will be distribution.Distribution")
}

func assertNoMetricsInChan(t *testing.T, metricChan <-chan telegraf.Metric) {
	select {
	case <-metricChan:
		assert.Fail(t, "We should not got any metrics yet")
	default:
	}
}
