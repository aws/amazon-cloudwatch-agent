package awscsm

import (
	"reflect"
	"time"
	"unsafe"

	"github.com/aws/amazon-cloudwatch-agent/awscsm/sdkmetricsdataplane"
)

const (
	// builtins
	sizePointer   = int64(unsafe.Sizeof(uintptr(0)))
	sizeTimestamp = int64(unsafe.Sizeof(time.Time{}))
	sizeString    = int64(unsafe.Sizeof(string("")))
	sizeInt64     = int64(unsafe.Sizeof(int64(0)))
	sizeFloat64   = int64(unsafe.Sizeof(float64(0)))

	// guess, close enough even if not fully accurate
	sizeSlice = int64(unsafe.Sizeof(reflect.SliceHeader{}))

	// API shapes, includes base shape plus any fixed sized primitive fields (int64, float64)
	sizeSdkMonitoringRecord    = int64(unsafe.Sizeof(sdkmetricsdataplane.SdkMonitoringRecord{})) + sizeInt64*2
	sizeSdkAggregationKey      = int64(unsafe.Sizeof(sdkmetricsdataplane.SdkAggregationKey{}))
	sizeSdkAggregationKeyEntry = int64(unsafe.Sizeof(sdkmetricsdataplane.SdkAggregationKeyEntry{}))
	sizeSehMetric              = int64(unsafe.Sizeof(sdkmetricsdataplane.SehMetric{}))
	sizeFrequencyMetric        = int64(unsafe.Sizeof(sdkmetricsdataplane.FrequencyMetric{}))

	sizeFrequencyDistributionEntry = int64(unsafe.Sizeof(sdkmetricsdataplane.FrequencyDistributionEntry{})) + sizeInt64
	sizeSehBucket                  = int64(unsafe.Sizeof(sdkmetricsdataplane.SehBucket{})) + sizeInt64 + sizeFloat64
	sizeStatisticSet               = int64(unsafe.Sizeof(sdkmetricsdataplane.StatisticSet{})) + 4*sizeFloat64
)

// It's worth trying to be accurate here as long as it doesn't cost much in performance.
// An accurate estimate lets us honor our memory usage promise while packing in as many
// records as we can during an outage.
//
// This function does not get called very often (several times a second
// under heavy usage) and so traversing the full data structures isn't a cause for alarm.
//
//
// Notes:
//
// For slices, we use the capacity rather than the length.  However, for slices
// of pointers, we use capacity * pointer_size + Sum(dereferenced individual element sizes).
// Due to this, you will see some calculations mixing both cap() and len() (or iteration)
//
// This function intentionally ignores alignment considerations (too complex, high likelihood of
// being incorrect or not helping)
func estimateRecordSize(record *sdkmetricsdataplane.SdkMonitoringRecord) int64 {

	if record == nil {
		return 0
	}

	// base record + primitive fields
	size := sizeSdkMonitoringRecord

	// String fields
	size += estimateStringSize(record.CompressedEventSamples)
	size += estimateStringSize(record.Id)
	size += estimateStringSize(record.Version)

	// Complex fields
	size += estimateAggregationKeySize(record.AggregationKey)
	size += estimateSehMetricsSize(record.SehMetrics)
	size += estimateFrequencyMetricsSize(record.FrequencyMetrics)

	return size
}

func estimateStringSize(value *string) int64 {
	if value == nil {
		return 0
	}

	return sizeString + int64(len(*value))
}

func estimateAggregationKeySize(aggregationKey *sdkmetricsdataplane.SdkAggregationKey) int64 {
	if aggregationKey == nil {
		return 0
	}

	// base structure size
	size := sizeSdkAggregationKey

	// timestamp
	size += sizeTimestamp

	// slice of pointers to key entries
	size += int64(cap(aggregationKey.Keys))*sizePointer + sizeSlice
	for _, k := range aggregationKey.Keys {
		size += estimateAggregationKeyEntrySize(k)
	}

	return size
}

func estimateAggregationKeyEntrySize(entry *sdkmetricsdataplane.SdkAggregationKeyEntry) int64 {
	if entry == nil {
		return 0
	}

	size := sizeSdkAggregationKeyEntry

	size += estimateStringSize(entry.Key)
	size += estimateStringSize(entry.Value)

	return size
}

func estimateSehMetricsSize(metrics []*sdkmetricsdataplane.SehMetric) int64 {
	size := int64(cap(metrics))*sizePointer + sizeSlice
	for _, m := range metrics {
		size += estimateSehMetricSize(m)
	}

	return size
}

func estimateSehMetricSize(metric *sdkmetricsdataplane.SehMetric) int64 {
	if metric == nil {
		return 0
	}

	size := sizeSehMetric + sizeStatisticSet
	size += estimateStringSize(metric.Name)
	size += int64(cap(metric.SehBuckets)) * sizePointer

	// assumes only the valid elements of the slice have valid pointers
	size += int64(len(metric.SehBuckets))*sizeSehBucket + sizeSlice

	return size
}

func estimateFrequencyMetricsSize(metrics []*sdkmetricsdataplane.FrequencyMetric) int64 {
	size := int64(cap(metrics))*sizePointer + sizeSlice
	for _, m := range metrics {
		size += estimateFrequencyMetricSize(m)
	}

	return size
}

func estimateFrequencyMetricSize(metric *sdkmetricsdataplane.FrequencyMetric) int64 {
	if metric == nil {
		return 0
	}

	size := sizeFrequencyMetric
	size += estimateStringSize(metric.Name)

	size += int64(cap(metric.Distribution))*sizePointer + sizeSlice
	for _, e := range metric.Distribution {
		size += estimateFrequencyDistributionEntrySize(e)
	}

	return size
}

func estimateFrequencyDistributionEntrySize(entry *sdkmetricsdataplane.FrequencyDistributionEntry) int64 {
	if entry == nil {
		return 0
	}

	// The Count(int64) field is rolled up into the base size constant of the struct
	size := sizeFrequencyDistributionEntry
	size += estimateStringSize(entry.Key)

	return size
}
