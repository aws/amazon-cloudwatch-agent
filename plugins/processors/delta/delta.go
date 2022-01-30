// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package delta

import (
	"log"
	"reflect"
	"strings"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/processors"
)

const (
	ReportDelta           string = "report_deltas"
	IgnoredFieldsForDelta string = "ignored_fields_for_delta"
	FieldSeparator        string = ","
	TrueValue             string = "true"
)

type metricFields struct {
	fields map[string]interface{}
}

type Delta struct {
	cache map[uint64]*metricFields
}

func copyMetricFields(metric telegraf.Metric) *metricFields {
	metricFieldsAndTime := metricFields{
		fields: make(map[string]interface{}),
	}
	for _, field := range metric.FieldList() {
		fv, ok := metric.GetField(field.Key)
		if ok {
			metricFieldsAndTime.fields[field.Key] = fv
		}
	}
	return &metricFieldsAndTime
}

var sampleConfig = `
`

func (d *Delta) SampleConfig() string {
	return sampleConfig
}

func (d *Delta) Description() string {
	return "Output the delta between current value and previous value."
}

func diff(num1 interface{}, num2 interface{}) interface{} {
	num1Int, ok1 := num1.(int64)
	num2Int, ok2 := num2.(int64)
	if ok1 && ok2 {
		return num1Int - num2Int
	}

	num1Uint, ok1 := num1.(uint64)
	num2Uint, ok2 := num2.(uint64)
	if ok1 && ok2 {
		return num1Uint - num2Uint
	}

	num1Float64, ok1 := num1.(float64)
	num2Float64, ok2 := num2.(float64)
	if ok1 && ok2 {
		return num1Float64 - num2Float64
	}

	log.Printf("E! system: Unexpected value types: %s, %s\n",
		reflect.TypeOf(num1), reflect.TypeOf(num2))
	return 0
}

func isIgnoredField(metric telegraf.Metric, fieldKey string) bool {
	ignored, ok := metric.GetTag(IgnoredFieldsForDelta)
	if ok {
		for _, ignoredField := range strings.Split(ignored, FieldSeparator) {
			if ignoredField == fieldKey {
				return true
			}
		}
	}
	return false
}

func (d *Delta) Apply(in ...telegraf.Metric) []telegraf.Metric {
	var result []telegraf.Metric

	for _, metric := range in {
		//delta doesn't apply to the current metric
		if tv, ok := metric.GetTag(ReportDelta); !ok || strings.ToLower(tv) != TrueValue {
			result = append(result, metric)
			continue
		}

		metricID := metric.HashID()
		lastMetric, ok := d.cache[metricID]

		//populate the cache for the first time
		if !ok {
			d.cache[metricID] = copyMetricFields(metric)
			continue
		}

		//update cache and modify original metric in place
		for _, field := range metric.FieldList() {
			fv, _ := metric.GetField(field.Key)
			last, ok := lastMetric.fields[field.Key]
			if ok && !isIgnoredField(metric, field.Key) {
				metric.AddField(field.Key, diff(fv, last))
			}
			d.cache[metricID].fields[field.Key] = fv
		}
		//remove the transient tags
		metric.RemoveTag(ReportDelta)
		metric.RemoveTag(IgnoredFieldsForDelta)

		result = append(result, metric)
	}

	return result
}

func init() {
	processors.Add("delta", func() telegraf.Processor {
		return &Delta{
			cache: make(map[uint64]*metricFields),
		}
	})
}
