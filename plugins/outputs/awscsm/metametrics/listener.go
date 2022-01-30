// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metametrics

import (
	"log"
	"math/rand"
	"time"

	awscsmmetrics "github.com/aws/amazon-cloudwatch-agent/awscsm"
)

const (
	aggregationIntervalPeriod = time.Minute
)

type MetricKey struct {
	Name      string
	Timestamp time.Time
	Endpoint  string
}

func NewMetricKeyForApiCall(name string, timestamp time.Time, endpoint string) MetricKey {
	return MetricKey{
		Name:      name,
		Timestamp: timestamp.Truncate(aggregationIntervalPeriod),
		Endpoint:  endpoint,
	}
}

// Metric represents a statistic set based distribution corresponding to agent api call performance
type Metric struct {
	Key   MetricKey
	Stats awscsmmetrics.StatisticSet
}

// Combines two metric distributions
func (m *Metric) Combine(other Metric) {
	m.Stats.Merge(other.Stats)
}

// Metrics is a map of metrics
type Metrics map[MetricKey]Metric

// MetricListener is a singleton that handles metrics about
// the agent during API calls to the sdk metrics dataplane and controlplane.
var MetricListener Listener

// Listener will listen for Metrics.
type Listener struct {
	Shutdown chan struct{}
	ch       chan Metric
	metrics  Metrics
	writer   MetricWriter
}

// NewListenerAndStart will return a new listener and instantiate all necessary
// fields. In addition, this will call the Listen method in a separate go
// routine.
func NewListenerAndStart(writer MetricWriter, size int, interval time.Duration) Listener {
	l := Listener{
		Shutdown: make(chan struct{}, 0),
		ch:       make(chan Metric, size),
		metrics:  Metrics{},
		writer:   writer,
	}

	go l.Listen(interval)

	return l
}

// Listen will run a loop until the Shutdown channel is closed. The
// loop will tick at every interval and publish any metrics in the
// container.
//
// The l.ch channel will wait until metrics are sent down the pipe
// and add to the metrics container.
func (l *Listener) Listen(interval time.Duration) {
	halfInterval := int64(interval) / 2

	t := time.NewTimer(interval)
	for {
		select {
		case <-l.Shutdown:
			l.flush()
			return

		case <-t.C:
			l.flush()

			newInterval := time.Duration(rand.Int63n(2 * halfInterval))
			newInterval += time.Duration(halfInterval)
			t.Reset(newInterval)

		case m := <-l.ch:
			cached := l.metrics[m.Key]
			cached.Key = m.Key
			cached.Combine(m)
			l.metrics[m.Key] = cached
		}
	}
}

func (l *Listener) flush() {
	if len(l.metrics) == 0 {
		return
	}

	oldMetrics := l.metrics
	l.metrics = make(Metrics)
	l.writer.Write(oldMetrics)
}

// Close will close the Shutdown channel shutting down the Listen method.
func (l *Listener) Close() {
	close(l.Shutdown)
	l.Shutdown = nil
}

// Count will construct a new Metric and send that in the metric
// channel.
func (l *Listener) Count(name string, value float64, timestamp time.Time, endpoint string) {
	if len(endpoint) == 0 {
		log.Println("Error - internal metrics referenced empty endpoint")
		return
	}

	l.ch <- Metric{
		Key:   NewMetricKeyForApiCall(name, timestamp, endpoint),
		Stats: awscsmmetrics.NewStatisticSet(value),
	}
}

// Convenience function for success rate metrics since there's no ternary operator to do it inline
func (l *Listener) CountSuccess(name string, success bool, timestamp time.Time, endpoint string) {
	value := 0.0
	if success {
		value = 1.0
	}

	l.Count(name, value, timestamp, endpoint)
}

// MetricWriter interface that is used to write a set of
// metrics
type MetricWriter interface {
	Write(Metrics) error
}
