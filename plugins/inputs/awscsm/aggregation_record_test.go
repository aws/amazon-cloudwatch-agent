// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscsm

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	awscsmmetrics "github.com/aws/amazon-cloudwatch-agent/awscsm"
	"github.com/aws/amazon-cloudwatch-agent/awscsm/csm"

	"github.com/aws/amazon-cloudwatch-agent/plugins/outputs/awscsm/providers"
)

func TestRecordConversion(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name     string
		records  AggregationRecords
		expected map[string]interface{}
	}{
		{
			name:     "empty case",
			records:  AggregationRecords{},
			expected: map[string]interface{}{},
		},
		{
			name: "only keys",
			records: AggregationRecords{
				"foo": AggregationRecord{},
				"Foo": AggregationRecord{},
				"bar": AggregationRecord{},
			},
			expected: map[string]interface{}{
				"foo": AggregationRecord{},
				"Foo": AggregationRecord{},
				"bar": AggregationRecord{},
			},
		},
		{
			name: "full test",
			records: AggregationRecords{
				"foo": AggregationRecord{
					Expiry: now,
				},
				"Foo": AggregationRecord{
					SEHMetrics: awscsmmetrics.SEHMetrics{
						"FOO": awscsmmetrics.SEHMetric{
							Buckets: map[int64]float64{
								0: 1.0,
							},
						},
					},
					FrequencyMetrics: awscsmmetrics.FrequencyMetrics{
						"BAR": awscsmmetrics.FrequencyMetric{
							Frequencies: map[string]int64{
								"int": 13,
							},
						},
					},
				},
				"bar": AggregationRecord{},
			},
			expected: map[string]interface{}{
				"foo": AggregationRecord{
					Expiry: now,
				},
				"Foo": AggregationRecord{
					SEHMetrics: awscsmmetrics.SEHMetrics{
						"FOO": awscsmmetrics.SEHMetric{
							Buckets: map[int64]float64{
								0: 1.0,
							},
						},
					},
					FrequencyMetrics: awscsmmetrics.FrequencyMetrics{
						"BAR": awscsmmetrics.FrequencyMetric{
							Frequencies: map[string]int64{
								"int": 13,
							},
						},
					},
				},
				"bar": AggregationRecord{},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if e, a := c.expected, c.records.MapStringInterface(); !reflect.DeepEqual(e, a) {
				t.Errorf("expected %v, but received %v", e, a)
			}
		})
	}
}

func TestBuildAggregationKey(t *testing.T) {
	now := time.Now()
	ms := float64(now.UnixNano() / int64(time.Millisecond))
	interval := int64(defaultIntervalPeriod / time.Millisecond)
	clamped := int64(ms) / interval * interval

	cases := []struct {
		name         string
		keyType      providers.EventEntryKeyType
		metric       map[string]interface{}
		expected     string
		expectedKeys map[string]string
	}{
		{
			name:    "empty case",
			keyType: providers.EventEntryKeyType(csm.MonitoringEventEntryKeyTypeAggregation),
			metric: map[string]interface{}{
				"Timestamp": ms,
			},
			expected: strconv.FormatInt(clamped, 10),
			expectedKeys: map[string]string{
				"Timestamp": strconv.FormatInt(clamped, 10),
			},
		},
		{
			name:    "partial case",
			keyType: providers.EventEntryKeyType(csm.MonitoringEventEntryKeyTypeAggregation),
			metric: map[string]interface{}{
				"ClientId":  "foo",
				"Timestamp": ms,
			},
			expected: strings.Join([]string{
				strconv.FormatInt(clamped, 10),
				"foo",
			}, sep),
			expectedKeys: map[string]string{
				"Timestamp": strconv.FormatInt(clamped, 10),
				"ClientId":  "foo",
			},
		},
		{
			name:    "full case",
			keyType: providers.EventEntryKeyType(csm.MonitoringEventEntryKeyTypeAggregation),
			metric: map[string]interface{}{
				"Timestamp": ms,
				"ClientId":  "foo",
				"Api":       "op",
				"Service":   "service",
				"Type":      "type",
			},
			expected: strings.Join([]string{
				strconv.FormatInt(clamped, 10),
				"op",
				"foo",
				"service",
				"type",
			}, sep),
			expectedKeys: map[string]string{
				"Timestamp": strconv.FormatInt(clamped, 10),
				"ClientId":  "foo",
				"Api":       "op",
				"Service":   "service",
				"Type":      "type",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			key, keys := buildAggregationKey(c.metric)
			if e, a := c.expected, key; e != a {
				t.Errorf("expected %q, but received %q", e, a)
			}

			if e, a := c.expectedKeys, keys; !reflect.DeepEqual(e, a) {
				t.Errorf("expected %v, but received %v", e, a)
			}
		})
	}
}

func init() {
	mock := &providers.MockConfigProvider{}
	providers.Config = mock
}
