// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscsm

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	awscsmmetrics "github.com/aws/amazon-cloudwatch-agent/awscsm"
	"github.com/aws/amazon-cloudwatch-agent/plugins/outputs/awscsm/providers"
)

// AggregationRecords is a map of aggregation records
// keyed off of each record's aggregation key
type AggregationRecords map[string]AggregationRecord

const (
	defaultSEHMetricWeight = 1.0

	errCountFormat = "failed to increment count for %s, %v"

	sep = "-"

	transformAggregationRecordDelay = time.Minute

	defaultIntervalPeriod = time.Minute

	msToSec = int64(time.Second / time.Millisecond)
	msToNs  = int64(time.Millisecond / time.Nanosecond)
)

// MapStringInterface will return a map[string]interface{} from the
// strongly typed AggregationRecords.
func (records AggregationRecords) MapStringInterface(filters ...AggregationRecordFilter) map[string]interface{} {
	m := map[string]interface{}{}
	for k, v := range records {
		toFilter := false
		for _, filter := range filters {
			if filter(v) {
				toFilter = true
				break
			}
		}

		if toFilter {
			continue
		}

		m[k] = v
		delete(records, k)
	}
	return m
}

// Add will add a metric to a given record by building an aggregation key
// from the given metric.
func (records AggregationRecords) Add(raw map[string]interface{}) {
	ts, ok := raw["Timestamp"]
	if !ok {
		return
	}

	key, keys := buildAggregationKey(raw)

	record, ok := records[key]
	if !ok {
		record = NewAggregationRecord()

		ms := int64(ts.(float64))
		t := time.Unix(ms/msToSec, (ms%msToSec)*msToNs)
		// TODO: 60 is the default. Will eventually be overriden by configuration

		record.Timestamp = t
		record.Keys = keys
	}

	aggIntervalEnd := record.Timestamp.Add(defaultIntervalPeriod)
	record.Expiry = max(time.Now(), aggIntervalEnd).Add(transformAggregationRecordDelay)
	cfg := providers.Config.RetrieveAgentConfig()

	eventType, ok := raw["Type"].(string)
	if !ok {
		log.Println(fmt.Sprintf("E! 'Type' needs to be a string: %T", raw["Type"]))
	}

	eventDef, hasEventDef := cfg.Definitions.Events.Get(eventType)
	for k, v := range raw {
		if hasEventDef && record.Samples.ShouldAdd(eventDef.SampleRate) {
			signature := getSampleSignature(cfg.Definitions, raw)
			if ok := record.Samples.Count(signature); !ok && record.Samples.Len() < eventDef.MaxSampleCount {
				record.Samples.Add(raw)
			}
		}

		def, ok := cfg.Definitions.Entries.Get(k)
		if !ok {
			continue
		}

		if def.Type.IsFrequency() {
			switch val := v.(type) {
			case string:
				record.addFrequency(k, val)
			case float64:
				record.addFrequency(k, strconv.Itoa(int(val)))
			default:
				continue
			}
		} else if def.Type.IsSEH() {
			switch val := v.(type) {
			case int:
				record.addSEH(k, float64(val), defaultSEHMetricWeight)
			case float64:
				record.addSEH(k, val, defaultSEHMetricWeight)
			}
		}

	}

	records[key] = record
}

func max(t1 time.Time, t2 time.Time) time.Time {
	if t1.Before(t2) {
		return t2
	}
	return t1
}

// AggregationRecord represents a SDK Metrics metric that will contain various metrics
// received from a client.
type AggregationRecord struct {
	Expiry    time.Time
	Timestamp time.Time
	Keys      map[string]string

	SEHMetrics       awscsmmetrics.SEHMetrics
	FrequencyMetrics awscsmmetrics.FrequencyMetrics
	Samples          Samples
}

// NewAggregationRecord will return an empty AggregationRecord
func NewAggregationRecord() AggregationRecord {
	return AggregationRecord{
		SEHMetrics:       awscsmmetrics.SEHMetrics{},
		FrequencyMetrics: awscsmmetrics.FrequencyMetrics{},
		Samples:          newSamples(),
	}
}

// GetFrequencyMetrics will return an array of FrequencyMetrics
func (m AggregationRecord) GetFrequencyMetrics() []awscsmmetrics.FrequencyMetric {
	metrics := []awscsmmetrics.FrequencyMetric{}
	for _, metric := range m.FrequencyMetrics {
		metrics = append(metrics, metric)
	}
	return metrics
}

// GetSEHMetrics will return an array of SEHMetric
func (m AggregationRecord) GetSEHMetrics() []awscsmmetrics.SEHMetric {
	metrics := []awscsmmetrics.SEHMetric{}
	for _, metric := range m.SEHMetrics {
		metrics = append(metrics, metric)
	}
	return metrics
}

// GetTimestamp will return the timestamp associated with the record
func (m AggregationRecord) GetTimestamp() time.Time {
	return m.Timestamp
}

// GetKeys will return the keys associated with the record
func (m AggregationRecord) GetKeys() map[string]string {
	return m.Keys
}

// GetSamples will return the list of samples gathered during metric
// collection.
func (m AggregationRecord) GetSamples() []map[string]interface{} {
	return m.Samples.list
}

func (m *AggregationRecord) addFrequency(name, key string) {
	cfg := providers.Config.RetrieveAgentConfig()
	if len(key) > cfg.Limits.MaxFrequencyDistributionKeySize {
		key = key[:cfg.Limits.MaxFrequencyDistributionKeySize]
	}

	entry, ok := m.FrequencyMetrics[name]
	if !ok {
		entry = awscsmmetrics.NewFrequencyMetric(name)
	}

	entry.CountSample(key)

	m.FrequencyMetrics[name] = entry
}

func (m *AggregationRecord) addSEH(name string, v, weight float64) {
	cfg := providers.Config.RetrieveAgentConfig()
	entry, ok := m.SEHMetrics[name]
	if !ok {
		entry = awscsmmetrics.NewSEHMetric(name)
	}

	if len(entry.Buckets) > cfg.Limits.MaxSEHBuckets {
		return
	}

	err := entry.AddWeightedSample(v, weight)
	if err != nil {
		log.Printf(errCountFormat, name, err)
		return
	}

	m.SEHMetrics[name] = entry
}

func buildAggregationKey(raw map[string]interface{}) (string, map[string]string) {
	keys, keyOrder := getKeys(raw)
	values := make([]string, len(keyOrder))

	for i := 0; i < len(keyOrder); i++ {
		key := keyOrder[i]
		v := keys[key]
		values[i] = v
	}

	return strings.Join(values, sep), keys
}

func getKeys(raw map[string]interface{}) (map[string]string, []string) {
	cfg := providers.Config.RetrieveAgentConfig()
	keys := map[string]string{}
	i := 1
	keyOrder := make([]string, len(raw))

	for k, v := range raw {
		def, ok := cfg.Definitions.Entries.Get(k)
		if !ok {
			continue
		}

		if def.KeyType.IsAggregation() {
			key := v.(string)
			if len(key) > cfg.Limits.MaxAggregationKeyValueSize {
				key = key[:cfg.Limits.MaxAggregationKeyValueSize]
			}
			keys[k] = key

			keyOrder[i] = k
			i++
		} else if def.KeyType.IsAggregationTimestamp() {
			interval := int64(defaultIntervalPeriod / time.Millisecond)
			t := int64(v.(float64))
			clampedTimestamp := t / interval
			clampedTimestamp *= interval

			keyOrder[0] = k
			keys[k] = strconv.FormatInt(clampedTimestamp, 10)
		}
	}

	if len(keyOrder) > 1 {
		sort.Strings(keyOrder[1:])
	}
	return keys, keyOrder
}
