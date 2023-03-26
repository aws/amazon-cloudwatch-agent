// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/metric/distribution"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/metric"
)

const (
	aggregationIntervalTagKey         = "aws:AggregationInterval"
	durationAggregationChanBufferSize = 10000
)

type Aggregator interface {
	AddMetric(m telegraf.Metric)
}

type aggregator struct {
	durationMap  map[time.Duration]*durationAggregator
	metricChan   chan<- telegraf.Metric
	shutdownChan <-chan struct{}
	wg           *sync.WaitGroup
}

func NewAggregator(metricChan chan<- telegraf.Metric, shutdownChan <-chan struct{}, wg *sync.WaitGroup) Aggregator {
	return &aggregator{
		durationMap:  make(map[time.Duration]*durationAggregator),
		metricChan:   metricChan,
		shutdownChan: shutdownChan,
		wg:           wg,
	}
}

func computeHash(m telegraf.Metric) string {
	tmp := make([]string, len(m.Tags()))
	i := 0
	for k, v := range m.Tags() {
		tmp[i] = fmt.Sprintf("%s=%s", k, v)
		i++
	}
	sort.Strings(tmp)
	return fmt.Sprintf("%s:%s", m.Name(), strings.Join(tmp, ","))
}

func (agg *aggregator) AddMetric(m telegraf.Metric) {
	var aggregationInterval string
	var ok bool
	if aggregationInterval, ok = m.Tags()[aggregationIntervalTagKey]; !ok {
		// no aggregation interval field key, pass through directly.
		agg.metricChan <- m
		return
	}

	// remove aggregation interval field key since it is irrelevant any more
	m.RemoveTag(aggregationIntervalTagKey)

	var aggregationDuration time.Duration
	var err error
	if aggregationDuration, err = time.ParseDuration(aggregationInterval); err != nil {
		log.Printf("W! aggregation interval string value %v cannot be parsed into time.Duration type. No aggregation will be performed. %v",
			aggregationInterval, err)
		agg.metricChan <- m
		return
	}

	aggDurationMapKey := aggregationDuration.Truncate(time.Second)
	var durationAgg *durationAggregator
	if durationAgg, ok = agg.durationMap[aggDurationMapKey]; !ok {
		durationAgg = newDurationAggregator(aggDurationMapKey, agg.metricChan, agg.shutdownChan, agg.wg)
		agg.durationMap[aggDurationMapKey] = durationAgg
	}

	//auto configure high resolution
	if aggDurationMapKey < time.Minute {
		m.AddTag(highResolutionTagKey, "true")
	}

	durationAgg.addMetric(m)
}

type durationAggregator struct {
	aggregationDuration time.Duration
	metricChan          chan<- telegraf.Metric
	shutdownChan        <-chan struct{}
	wg                  *sync.WaitGroup
	ticker              *time.Ticker
	metricMap           map[string]telegraf.Metric //metric hash string + time sec int64 -> Metric object
	aggregationChan     chan telegraf.Metric
}

func newDurationAggregator(durationInSeconds time.Duration,
	metricChan chan<- telegraf.Metric,
	shutdownChan <-chan struct{},
	wg *sync.WaitGroup) *durationAggregator {

	durationAgg := &durationAggregator{
		aggregationDuration: durationInSeconds,
		metricChan:          metricChan,
		shutdownChan:        shutdownChan,
		wg:                  wg,
		metricMap:           make(map[string]telegraf.Metric),
		aggregationChan:     make(chan telegraf.Metric, durationAggregationChanBufferSize),
	}

	go durationAgg.aggregating()

	return durationAgg
}

func (durationAgg *durationAggregator) aggregating() {
	durationAgg.wg.Add(1)
	// Sleep to align the interval to the wall clock.
	// This initial sleep is not interrupted if the aggregator gets shutdown.
	now := time.Now()
	time.Sleep(now.Truncate(durationAgg.aggregationDuration).Add(durationAgg.aggregationDuration).Sub(now))
	durationAgg.ticker = time.NewTicker(durationAgg.aggregationDuration)
	defer durationAgg.ticker.Stop()
	for {
		// There is no priority to select{}.
		// If there is a new metric AND the shutdownChan is closed when this
		// loop begins, then the behavior is random.
		select {
		case m := <-durationAgg.aggregationChan:
			// https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_MetricDatum.html
			aggregatedTime := m.Time().Truncate(durationAgg.aggregationDuration)
			metricMapKey := fmt.Sprint(computeHash(m), aggregatedTime.Unix())
			var aggregatedMetric telegraf.Metric
			var ok bool
			if aggregatedMetric, ok = durationAgg.metricMap[metricMapKey]; !ok {
				aggregatedMetric = metric.New(m.Name(), m.Tags(), map[string]interface{}{}, aggregatedTime)
				durationAgg.metricMap[metricMapKey] = aggregatedMetric
			}
			//When the code comes here, it means the aggregatedMetric object has the same metric name, tags and aggregated time.
			//We just need to aggregate the additional fields if any and the values for the fields.
			for k, v := range m.Fields() {
				var value float64
				var dist distribution.Distribution
				switch t := v.(type) {
				case int:
					value = float64(t)
				case int32:
					value = float64(t)
				case int64:
					value = float64(t)
				case float64:
					value = t
				case bool:
					if t {
						value = 1
					} else {
						value = 0
					}
				case time.Time:
					value = float64(t.Unix())
				case distribution.Distribution:
					dist = t
				default:
					// Skip unsupported type.
					continue
				}
				var existingValue interface{}
				if existingValue, ok = aggregatedMetric.Fields()[k]; !ok {
					existingValue = distribution.NewDistribution()
					aggregatedMetric.AddField(k, existingValue)
				}
				existingDist := existingValue.(distribution.Distribution)
				if dist != nil {
					existingDist.AddDistribution(dist)
				} else {
					err := existingDist.AddEntry(value, 1)
					if err != nil {
						log.Printf("W! error: %s, metric %s, value %v", err, m.Name(), value)
					}
				}
			}
		case <-durationAgg.ticker.C:
			durationAgg.flush()
		case <-durationAgg.shutdownChan:
			log.Printf("D! CloudWatch: aggregating routine receives the shutdown signal, do the final flush now for aggregation interval %v", durationAgg.aggregationDuration)
			durationAgg.flush()
			log.Printf("D! CloudWatch: aggregating routine receives the shutdown signal, exiting.")
			durationAgg.wg.Done()
			return
		}
	}
}

func (durationAgg *durationAggregator) addMetric(m telegraf.Metric) {
	durationAgg.aggregationChan <- m
}

func (durationAgg *durationAggregator) flush() {
	for _, v := range durationAgg.metricMap {
		durationAgg.metricChan <- v
	}
	durationAgg.metricMap = make(map[string]telegraf.Metric)
}
