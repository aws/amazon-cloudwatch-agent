// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/service/cloudwatch"

	"github.com/aws/amazon-cloudwatch-agent/metric/distribution"
)

const (
	aggregationIntervalTagKey         = "aws:AggregationInterval"
	durationAggregationChanBufferSize = 10000
)

// aggregationDatum just adds a few extra fields to the MetricDatum.
// If aggregationInterval is 0, then no aggregation is done.
// If receivers set the special attribute "aws:AggregationInterval", then
// this exporter will remove it and do aggregation.
type aggregationDatum struct {
	cloudwatch.MetricDatum
	aggregationInterval time.Duration
	distribution        distribution.Distribution
}

type Aggregator interface {
	AddMetric(m *aggregationDatum)
}

var _ Aggregator = (*aggregator)(nil)

type aggregator struct {
	durationMap  map[time.Duration]*durationAggregator
	metricChan   chan<- *aggregationDatum
	shutdownChan <-chan struct{}
	wg           *sync.WaitGroup
}

func NewAggregator(metricChan chan<- *aggregationDatum, shutdownChan <-chan struct{}, wg *sync.WaitGroup) Aggregator {
	return &aggregator{
		durationMap:  make(map[time.Duration]*durationAggregator),
		metricChan:   metricChan,
		shutdownChan: shutdownChan,
		wg:           wg,
	}
}

func getAggregationKey(m *aggregationDatum, unixTime int64) string {
	tmp := make([]string, len(m.Dimensions))
	for i, d := range m.Dimensions {
		if d.Name == nil || d.Value == nil {
			log.Printf("E! dimentions key and/or val is nil")
			continue
		}
		tmp[i] = fmt.Sprintf("%s=%s", *d.Name, *d.Value)
	}
	// Assume m.Dimensions was already sorted.
	return fmt.Sprintf("%s:%s:%v", *m.MetricName, strings.Join(tmp, ","), unixTime)
}

func (agg *aggregator) AddMetric(m *aggregationDatum) {
	if m.aggregationInterval == 0 {
		// no aggregation interval field key, pass through directly.
		agg.metricChan <- m
		return
	}
	aggDurationMapKey := m.aggregationInterval.Truncate(time.Second)
	durationAgg, ok := agg.durationMap[aggDurationMapKey]
	if !ok {
		durationAgg = newDurationAggregator(aggDurationMapKey, agg.metricChan, agg.shutdownChan, agg.wg)
		agg.durationMap[aggDurationMapKey] = durationAgg
	}
	// auto configure high resolution
	if aggDurationMapKey < time.Minute {
		m.SetStorageResolution(1)
	}
	durationAgg.addMetric(m)
}

type durationAggregator struct {
	aggregationDuration time.Duration
	metricChan          chan<- *aggregationDatum
	shutdownChan        <-chan struct{}
	wg                  *sync.WaitGroup
	ticker              *time.Ticker
	// metric hash string + time sec int64 -> Metric object
	metricMap       map[string]*aggregationDatum
	aggregationChan chan *aggregationDatum
}

func newDurationAggregator(durationInSeconds time.Duration,
	metricChan chan<- *aggregationDatum,
	shutdownChan <-chan struct{},
	wg *sync.WaitGroup) *durationAggregator {

	durationAgg := &durationAggregator{
		aggregationDuration: durationInSeconds,
		metricChan:          metricChan,
		shutdownChan:        shutdownChan,
		wg:                  wg,
		metricMap:           make(map[string]*aggregationDatum),
		aggregationChan:     make(chan *aggregationDatum, durationAggregationChanBufferSize),
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
			if m == nil || m.Timestamp == nil || m.MetricName == nil || m.Unit == nil {
				log.Printf("E! cannot aggregate nil or partial datum")
				continue
			}
			// https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_MetricDatum.html
			aggregatedTime := m.Timestamp.Truncate(durationAgg.aggregationDuration)
			metricMapKey := getAggregationKey(m, aggregatedTime.Unix())
			aggregatedMetric, ok := durationAgg.metricMap[metricMapKey]
			if !ok {
				// First entry. Initialize it.
				durationAgg.metricMap[metricMapKey] = m
				if m.distribution == nil {
					// Assume function pointer is always valid.
					m.distribution = distribution.NewDistribution()
					err := m.distribution.AddEntryWithUnit(*m.Value, 1, *m.Unit)
					if err != nil {
						if errors.Is(err, distribution.ErrUnsupportedValue) {
							log.Printf("W! err %s, metric %s", err, *m.MetricName)
						} else {
							log.Printf("D! err %s, metric %s", err, *m.MetricName)
						}
					}
				}
				// Else the first entry has a distribution, so do nothing.
			} else {
				// Update an existing entry.
				if m.distribution == nil {
					err := aggregatedMetric.distribution.AddEntryWithUnit(*m.Value, 1, *m.Unit)
					if err != nil {
						log.Printf("W! err %s, metric %s", err, *m.MetricName)
					}
				} else {
					aggregatedMetric.distribution.AddDistribution(m.distribution)
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

func (durationAgg *durationAggregator) addMetric(m *aggregationDatum) {
	durationAgg.aggregationChan <- m
}

func (durationAgg *durationAggregator) flush() {
	for _, v := range durationAgg.metricMap {
		durationAgg.metricChan <- v
	}
	durationAgg.metricMap = make(map[string]*aggregationDatum)
}
