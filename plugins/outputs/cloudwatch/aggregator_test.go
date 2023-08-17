// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/metric/distribution"
	"github.com/aws/amazon-cloudwatch-agent/metric/distribution/seh1"
)

var wg sync.WaitGroup

func TestAggregator_NoAggregationKeyFound(t *testing.T) {
	metricChan, shutdownChan, aggregator := testPreparation()
	// no aggregation key found
	tags := map[string]string{"d1key": "d1value", "d2key": "d2value"}
	m := makeTestMetric("value", 1, time.Now(), tags, 0, "Percent")

	aggregator.AddMetric(m)
	select {
	case aggregatedMetric := <-metricChan:
		assert.Equal(t, time.Duration(0), aggregatedMetric.aggregationInterval)
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
	tags := map[string]string{"d1key": "d1value", "d2key": "d2value"}
	m := makeTestMetric("mname", 1, time.Now(), tags, aggregationInterval, "Percent")
	aggregator.AddMetric(m)

	assertNoMetricsInChan(t, metricChan)
	d := distribution.NewDistribution()
	d.AddEntryWithUnit(1, 1, "Percent")
	want := map[string]distribution.Distribution{"mname": d}
	testCheckMetrics(t, metricChan, 3*aggregationInterval, want)
	assertNoMetricsInChan(t, metricChan)
	close(shutdownChan)
	// Cleanup
	wg.Wait()
}

func TestAggregator_MultipleAggregationPeriods(t *testing.T) {
	metricChan, shutdownChan, aggregator := testPreparation()
	timestamp := time.Now()
	tags := map[string]string{"d1key": "d1value", "d2key": "d2value"}
	// Multiple metrics with multiple aggregation period, some aggregation period are the same, some are not.
	metrics := []*aggregationDatum{
		makeTestMetric("mytrick", 1, timestamp, tags, time.Second, "Percent"),
		makeTestMetric("mytrick", 2, timestamp, tags, time.Second, "Percent"),
		makeTestMetric("mytrick", 3, timestamp, tags, time.Second, "Percent"),
		// different interval
		makeTestMetric("mytrick", 4, timestamp, tags, 2*time.Second, "Percent"),
		// different metric name
		makeTestMetric("metrique", 1, timestamp, tags, 2*time.Second, "Percent"),
		// back to the original name
		makeTestMetric("mytrick", 5, timestamp, tags, 2*time.Second, "Percent"),
		makeTestMetric("metrique", 2, timestamp, tags, 2*time.Second, "Percent"),
	}
	for _, m := range metrics {
		aggregator.AddMetric(m)
	}

	assertNoMetricsInChan(t, metricChan)
	// Expect just 1 datum at the 1 second interval.
	d := distribution.NewDistribution()
	d.AddEntryWithUnit(1, 1, "Percent")
	d.AddEntryWithUnit(2, 1, "Percent")
	d.AddEntryWithUnit(3, 1, "Percent")
	want := map[string]distribution.Distribution{"mytrick": d}
	testCheckMetrics(t, metricChan, 4*time.Second, want)
	assertNoMetricsInChan(t, metricChan)
	// Expect 2 datums at the 2 second interval.
	d = distribution.NewDistribution()
	d.AddEntryWithUnit(4, 1, "Percent")
	d.AddEntryWithUnit(5, 1, "Percent")
	d2 := distribution.NewDistribution()
	d2.AddEntryWithUnit(1, 1, "Percent")
	d2.AddEntryWithUnit(2, 1, "Percent")
	want = map[string]distribution.Distribution{"mytrick": d, "metrique": d2}
	testCheckMetrics(t, metricChan, 4*time.Second, want)
	assertNoMetricsInChan(t, metricChan)
	close(shutdownChan)
	// Cleanup
	wg.Wait()
}

func TestAggregator_ShutdownBehavior(t *testing.T) {
	metricChan, shutdownChan, aggregator := testPreparation()
	// Verify the remaining metrics can be read after shutdown.
	// The metrics should be available immediately after the shutdown even before aggregation period.
	aggregationInterval := 2 * time.Second
	tags := map[string]string{"d7key": "d7value", "d9key": "d9value"}
	timestamp := time.Now()
	m := makeTestMetric("mname1", 1, timestamp, tags, aggregationInterval, "Percent")
	aggregator.AddMetric(m)
	// The Aggregator creates a new durationAggregator for each metric.
	// And there is a delay when each new durationAggregator begins.
	// So submit a metric and wait for the first aggregation to occur.
	d := distribution.NewDistribution()
	d.AddEntryWithUnit(1, 1, "Percent")
	want := map[string]distribution.Distribution{"mname1": d}
	testCheckMetrics(t, metricChan, 3*aggregationInterval, want)
	assertNoMetricsInChan(t, metricChan)
	// Now submit the same metric and it should be routed to the existing
	// durationAggregator without delay.
	timestamp = time.Now()
	m = makeTestMetric("mname1", 1, timestamp, tags, aggregationInterval, "Percent")
	aggregator.AddMetric(m)
	// Shutdown before the 2nd aggregationInterval completes.
	close(shutdownChan)
	wg.Wait()
	testCheckMetrics(t, metricChan, time.Second, want)
	assertNoMetricsInChan(t, metricChan)
}

// TestDurationAggregator_aggregating verifies the metric's timetstamp is used to aggregate.
// If the same metric appears multiple times in a single aggregation interval then just expect 1 aggregated metric.
// If the same metric appears multiple times in different aggregation intervals then expect multiple aggregated metrics.
func TestDurationAggregator_aggregating(t *testing.T) {
	distribution.NewDistribution = seh1.NewSEH1Distribution
	aggregationInterval := 1 * time.Second
	shutdownChan := make(chan struct{})
	metricChan := make(chan *aggregationDatum, metricChanBufferSize)
	durationAgg := &durationAggregator{
		aggregationDuration: aggregationInterval,
		metricChan:          metricChan,
		shutdownChan:        shutdownChan,
		wg:                  &wg,
		metricMap:           make(map[string]*aggregationDatum),
		aggregationChan:     make(chan *aggregationDatum, durationAggregationChanBufferSize),
	}
	go durationAgg.aggregating()

	timestamp := time.Now()
	tags := map[string]string{"d1key": "d1value", "d2key": "d2value"}
	metrics := []*aggregationDatum{
		makeTestMetric("metrisch", 1, timestamp, tags, time.Second, "Percent"),
		makeTestMetric("metrisch", 1, timestamp.Add(aggregationInterval), tags, time.Second, "Percent"),
		makeTestMetric("metrisch", 2, timestamp, tags, time.Second, "Percent"),
		makeTestMetric("metrisch", 2, timestamp.Add(2*aggregationInterval), tags, time.Second, "Percent"),
		makeTestMetric("metrisch", 3, timestamp, tags, time.Second, "Percent"),
		makeTestMetric("metrisch", 3, timestamp.Add(3*aggregationInterval), tags, time.Second, "Percent"),
	}

	for _, m := range metrics {
		durationAgg.addMetric(m)
	}

	//give some time to aggregation to do the work
	time.Sleep(time.Second + 3*aggregationInterval)
	close(shutdownChan)
	wg.Wait()
	assert.Empty(t, durationAgg.aggregationChan)
	assert.Empty(t, durationAgg.metricMap)
	assert.Equal(t, 4, len(durationAgg.metricChan))
	close(durationAgg.metricChan)
}

func testPreparation() (chan *aggregationDatum, chan struct{}, Aggregator) {
	distribution.NewDistribution = seh1.NewSEH1Distribution
	metricChan := make(chan *aggregationDatum, metricChanBufferSize)
	shutdownChan := make(chan struct{})
	aggregator := NewAggregator(metricChan, shutdownChan, &wg)
	return metricChan, shutdownChan, aggregator
}

// makeTestMetric is a test helper function that constructs a datum.
func makeTestMetric(
	name string,
	value float64,
	ts time.Time,
	tags map[string]string,
	aggregationInterval time.Duration,
	unit string,
) *aggregationDatum {
	sortedKeys := sortedTagKeys(tags)
	dims := []*cloudwatch.Dimension{}
	for _, k := range sortedKeys {
		v := tags[k]
		d := cloudwatch.Dimension{}
		d.SetName(k)
		d.SetValue(v)
		dims = append(dims, &d)
	}
	ag := aggregationDatum{}
	ag.SetMetricName(name)
	ag.SetValue(value)
	ag.SetTimestamp(ts)
	ag.SetDimensions(dims)
	ag.SetUnit(unit)
	ag.aggregationInterval = aggregationInterval
	return &ag
}

// testCheckMetrics waits for aggregated datums, then validated the distribution
// in the datum.
func testCheckMetrics(
	t *testing.T,
	metricChan <-chan *aggregationDatum,
	metricMaxWait time.Duration,
	want map[string]distribution.Distribution,
) {
	for _, _ = range want {
		var got *aggregationDatum
		t.Log("Waiting for metric.")
		select {
		case got = <-metricChan:
		case <-time.After(metricMaxWait):
			assert.FailNow(t, "We should've seen 1 metric by now")
		}
		t.Log("Checking metric.")
		w, ok := want[*got.MetricName]
		if !ok {
			assert.FailNowf(t, "unexpected metric received, %s", *got.MetricName)
			continue
		}

		assert.Equal(t, w.Maximum(), got.distribution.Maximum())
		assert.Equal(t, w.SampleCount(), got.distribution.SampleCount())
		assert.Equal(t, w.Unit(), got.distribution.Unit())
		assert.Equal(t, w.Minimum(), got.distribution.Minimum())
		assert.Equal(t, w.Sum(), got.distribution.Sum())

		wantVals, wantCounts := w.ValuesAndCounts()
		gotVals, gotCounts := got.distribution.ValuesAndCounts()
		// sort the vals and counts.
		sort.Float64s(wantVals)
		sort.Float64s(wantCounts)
		sort.Float64s(gotVals)
		sort.Float64s(gotCounts)
		assert.Equal(t, wantVals, gotVals)
		assert.Equal(t, wantCounts, gotCounts)
	}

}

func assertNoMetricsInChan(t *testing.T, metricChan <-chan *aggregationDatum) {
	select {
	case <-metricChan:
		assert.Fail(t, "We should not got any metrics yet")
	default:
	}
}
