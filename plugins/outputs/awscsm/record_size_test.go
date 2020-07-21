// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscsm

import (
	"testing"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/awscsm/sdkmetricsdataplane"
)

const (
	sampleSize = 2000

	aggKeyEntryKeySize   = 100
	aggKeyEntryValueSize = 125
	aggKeyCount          = 5
	sehBucketCount       = 15
	frequencyEntryCount  = 15
)

// A collection of loose tests that check if the size of estimate of a record is at least as big
// as the non-empty changes to the record

func TestRecordEmpty(t *testing.T) {
	recordSize := estimateRecordSize(&sdkmetricsdataplane.SdkMonitoringRecord{})

	if recordSize < 0 {
		t.Errorf("Expected non-negative record size for empty record, observed %v", recordSize)
	}
}

func TestRecordFields(t *testing.T) {
	version := "Sadffsadfsadgasgasg"
	id := "fsdajkflajklsdfsdfasdfasdfasdfasdfasdfasdf"

	record := sdkmetricsdataplane.SdkMonitoringRecord{
		Version: &version,
		Id:      &id,
	}

	recordSize := estimateRecordSize(&record)
	minimumExpectedSize := int64(len(version) + len(id))
	if recordSize < minimumExpectedSize {
		t.Errorf("Expected record size estimate (%v) to be larger than the sum of its string field lengths (%v)", recordSize, minimumExpectedSize)
	}
}

func TestRecordSamples(t *testing.T) {
	samples := string(make([]rune, sampleSize))

	record := sdkmetricsdataplane.SdkMonitoringRecord{
		CompressedEventSamples: &samples,
	}

	recordSize := estimateRecordSize(&record)
	if recordSize < sampleSize {
		t.Errorf("Expected record size estimate (%v) to be larger than its sample length (%v)", recordSize, sampleSize)
	}
}

func TestRecordAggregationKey(t *testing.T) {
	key := string(make([]rune, aggKeyEntryKeySize))
	value := string(make([]rune, aggKeyEntryValueSize))
	keys := make([]*sdkmetricsdataplane.SdkAggregationKeyEntry, aggKeyCount)
	for i := 0; i < aggKeyCount; i++ {
		entry := sdkmetricsdataplane.SdkAggregationKeyEntry{
			Key:   &key,
			Value: &value,
		}
		keys[i] = &entry
	}

	now := time.Now()
	aggregationKey := sdkmetricsdataplane.SdkAggregationKey{
		Timestamp: &now,
		Keys:      keys,
	}

	record := sdkmetricsdataplane.SdkMonitoringRecord{
		AggregationKey: &aggregationKey,
	}

	recordSize := estimateRecordSize(&record)
	minimumExpectedSize := int64(aggKeyCount * (len(key) + len(value)))
	if recordSize < minimumExpectedSize {
		t.Errorf("Expected record size estimate (%v) to be larger than the sum of its aggregation key entry lengths (%v)", recordSize, minimumExpectedSize)
	}
}

func TestSehMetric(t *testing.T) {
	buckets := make([]*sdkmetricsdataplane.SehBucket, sehBucketCount)
	for i := 0; i < sehBucketCount; i++ {
		bucket := sdkmetricsdataplane.SehBucket{}
		buckets[i] = &bucket
	}

	name := "TestMetric"
	sehDistribution := sdkmetricsdataplane.SehMetric{
		Name:       &name,
		SehBuckets: buckets,
	}

	record := sdkmetricsdataplane.SdkMonitoringRecord{
		SehMetrics: []*sdkmetricsdataplane.SehMetric{&sehDistribution},
	}

	recordSize := estimateRecordSize(&record)
	minimumExpectedSize := sehBucketCount*(sizeSehBucket+sizePointer) + int64(len(name))

	if recordSize < minimumExpectedSize {
		t.Errorf("Expected record size estimate (%v) to be larger than the minimum size of the Seh distribution (%v)", recordSize, minimumExpectedSize)
	}
}

func TestFrequencyMetric(t *testing.T) {
	entries := make([]*sdkmetricsdataplane.FrequencyDistributionEntry, frequencyEntryCount)
	key := "FrequencyDistributionKey"

	for i := 0; i < frequencyEntryCount; i++ {
		entry := sdkmetricsdataplane.FrequencyDistributionEntry{
			Key: &key,
		}
		entries[i] = &entry
	}

	name := "TestMetric"
	frequencyDistribution := sdkmetricsdataplane.FrequencyMetric{
		Name:         &name,
		Distribution: entries,
	}

	record := sdkmetricsdataplane.SdkMonitoringRecord{
		FrequencyMetrics: []*sdkmetricsdataplane.FrequencyMetric{&frequencyDistribution},
	}

	recordSize := estimateRecordSize(&record)
	minimumExpectedSize := frequencyEntryCount*(int64(len(key))+sizePointer) + int64(len(name))

	if recordSize < minimumExpectedSize {
		t.Errorf("Expected record size estimate (%v) to be larger than the minimum size of the frequency distribution (%v)", recordSize, minimumExpectedSize)
	}
}
