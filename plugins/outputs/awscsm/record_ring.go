// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscsm

import (
	"container/list"

	awscsmmetrics "github.com/aws/amazon-cloudwatch-agent/awscsm"
	"github.com/aws/amazon-cloudwatch-agent/awscsm/sdkmetricsdataplane"

	"github.com/influxdata/telegraf"
)

type recordRing struct {
	sizeLimitInBytes   int64
	currentSizeInBytes int64
	records            *list.List
}

func newRecordRing(maxSizeInBytes int64) recordRing {
	return recordRing{
		sizeLimitInBytes: maxSizeInBytes,
		records:          list.New(),
	}
}

func (l *recordRing) pushFront(record *sdkmetricsdataplane.SdkMonitoringRecord) {
	recordSize := estimateRecordSize(record)

	for l.currentSizeInBytes+recordSize > l.sizeLimitInBytes && !l.empty() {
		l.popBack()
	}

	// add record to end and update size total
	l.currentSizeInBytes += recordSize
	l.records.PushFront(record)
}

func (l *recordRing) empty() bool {
	return l.records.Len() == 0
}

func (l *recordRing) popFront() *sdkmetricsdataplane.SdkMonitoringRecord {
	frontRecord := l.records.Front().Value.(*sdkmetricsdataplane.SdkMonitoringRecord)
	l.currentSizeInBytes -= estimateRecordSize(frontRecord)
	l.records.Remove(l.records.Front())

	return frontRecord
}

func (l *recordRing) popBack() {
	backRecord := l.records.Back().Value.(*sdkmetricsdataplane.SdkMonitoringRecord)
	l.currentSizeInBytes -= estimateRecordSize(backRecord)
	l.records.Remove(l.records.Back())
}

// exists to simplify parts of the awscsm tests
func (l *recordRing) toSlice() []*sdkmetricsdataplane.SdkMonitoringRecord {
	slice := []*sdkmetricsdataplane.SdkMonitoringRecord{}

	for e := l.records.Back(); e != nil; e = e.Prev() {
		slice = append(slice, e.Value.(*sdkmetricsdataplane.SdkMonitoringRecord))
	}

	return slice
}

func buildMetrics(m telegraf.Metric) []awscsmmetrics.Metric {
	metrics := []awscsmmetrics.Metric{}
	for _, v := range m.Fields() {
		fMetric, ok := v.(awscsmmetrics.Metric)
		if !ok {
			continue
		}

		metrics = append(metrics, fMetric)
	}
	return metrics
}
