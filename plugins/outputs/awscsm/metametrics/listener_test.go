// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metametrics_test

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	awscsmmetrics "github.com/aws/amazon-cloudwatch-agent/awscsm"
	"github.com/aws/amazon-cloudwatch-agent/plugins/outputs/awscsm/metametrics"
)

type mockWriter struct {
	WriteFn func(metametrics.Metrics) error
}

func (writer *mockWriter) Write(metrics metametrics.Metrics) error {
	if writer.WriteFn == nil {
		return nil
	}

	return writer.WriteFn(metrics)
}

func TestStartAndShutdown(t *testing.T) {
	writer := &mockWriter{}
	listener := metametrics.NewListenerAndStart(writer, 1, 15*time.Minute)
	if listener.Shutdown == nil {
		t.Errorf("failed to create new listener")
	}

	listener.Close()
	if listener.Shutdown != nil {
		t.Errorf("failed to shutdown listener")
	}
}

func TestMetricWriter(t *testing.T) {
	uniqueTimestamps := 5
	innerMetricCount := 100
	innerMetricBins := 11

	sentinelEndpoint := "test.com"
	sentinelName := "Sentinel"
	testEndpoints := []string{
		sentinelEndpoint,
		"test2.com",
	}

	metrics := metametrics.Metrics{}
	wg := sync.WaitGroup{}
	wg.Add(1)

	// previously, we were just counting signals to determine when the channel was empty and all data processed, but
	// that doesn't work with aggregation (count target isn't predicatable)
	// So instead, detect empty/finished state by sending a sentinel value at the end (channel preserving order) and
	// have the mock writer recognize the sentinel value and do a single signal/wait

	writer := &mockWriter{
		WriteFn: func(vals metametrics.Metrics) error {

			for k, v := range vals {
				// if it's the last value, then signal we're done on exit
				if k.Name == sentinelName {
					defer wg.Done()
				}
				existing := metrics[k]
				existing.Key = k
				existing.Combine(v)
				metrics[k] = existing
			}

			return nil
		},
	}

	now := time.Now()
	truncatedNow := now.Truncate(time.Minute)

	listener := metametrics.NewListenerAndStart(writer, 13, 500*time.Millisecond)
	for k := 0; k < len(testEndpoints); k++ {
		for j := 0; j < uniqueTimestamps; j++ {
			for i := 0; i < innerMetricCount; i++ {
				eventTime := now.Add(time.Duration(j) * time.Minute)
				bin := i % innerMetricBins
				t.Logf("%d", bin)
				listener.Count(fmt.Sprintf("%d", bin), float64(bin), eventTime, testEndpoints[k])
			}
		}
	}
	listener.Count(sentinelName, 0, now, sentinelEndpoint)

	wg.Wait()
	listener.Close()

	expected := metametrics.Metrics{}
	for k := 0; k < len(testEndpoints); k++ {
		for j := 0; j < uniqueTimestamps; j++ {
			for i := 0; i < innerMetricBins; i++ {
				key := metametrics.MetricKey{
					Name:      fmt.Sprintf("%d", i),
					Timestamp: truncatedNow.Add(time.Duration(j) * time.Minute),
					Endpoint:  testEndpoints[k],
				}

				expectedBinCount := innerMetricCount / innerMetricBins
				if i < innerMetricCount%innerMetricBins {
					expectedBinCount++
				}

				expected[key] = metametrics.Metric{
					Key: key,
					Stats: awscsmmetrics.StatisticSet{
						SampleCount: float64(expectedBinCount),
						Sum:         float64(expectedBinCount * i),
						Min:         float64(i),
						Max:         float64(i),
					},
				}
			}
		}
	}

	sentinelKey := metametrics.MetricKey{
		Name:      sentinelName,
		Timestamp: truncatedNow,
		Endpoint:  sentinelEndpoint,
	}

	sentinelMetric := metametrics.Metric{
		Key: sentinelKey,
		Stats: awscsmmetrics.StatisticSet{
			SampleCount: 1.0,
			Sum:         0.0,
			Min:         0.0,
			Max:         0.0,
		},
	}

	expected[sentinelKey] = sentinelMetric

	if len(metrics) != len(expected) {
		t.Errorf("expected %d, but received %d", len(expected), len(metrics))
	}

	if e, a := expected, metrics; !reflect.DeepEqual(e, a) {
		t.Errorf("expected %v\n---------------\nreceived %v", e, a)
	}
}
