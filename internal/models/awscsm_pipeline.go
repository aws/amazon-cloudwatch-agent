// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package models

import (
	"log"
	"math"

	awscsmmetrics "github.com/aws/amazon-cloudwatch-agent/awscsm"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/selfstat"
)

var GlobalMetricsGathered = selfstat.Register("agent", "metrics_gathered", map[string]string{})

var (
	AwsCsmInputChannel  = make(chan telegraf.Metric, 1000)
	AwsCsmOutputChannel = make(chan awscsmmetrics.Metric, 1000)
)

type AwsCsmMakeMetric struct {
}

func (r *AwsCsmMakeMetric) Name() string {
	return "awscsmMetric"
}

//TODO: we add this new method to satisfy agent.MetricMaker interface
//we need to investigate what logger is proper here
func (r *AwsCsmMakeMetric) LogName() string {
	return "awscsmMetric"
}

//TODO: we add this new method to satisfy agent.MetricMaker interface
//we need to investigate what logger is proper here
func (r *AwsCsmMakeMetric) Log() telegraf.Logger {
	return nil
}

func updateField(m telegraf.Metric, key string, value interface{}) {
	m.RemoveField(key)
	m.AddField(key, value)
}

//the implementation is similar to MakeMetric function in model/running_input.go
func (r *AwsCsmMakeMetric) MakeMetric(m telegraf.Metric) telegraf.Metric {
	for k, v := range m.Fields() {
		// Validate uint64 and float64 fields
		// convert all int & uint types to int64
		switch val := v.(type) {
		case nil:
			// delete nil fields
			m.RemoveField(k)
		case uint:
			updateField(m, k, int64(val))
			continue
		case uint8:
			updateField(m, k, int64(val))
			continue
		case uint16:
			updateField(m, k, int64(val))
			continue
		case uint32:
			updateField(m, k, int64(val))
			continue
		case int:
			updateField(m, k, int64(val))
			continue
		case int8:
			updateField(m, k, int64(val))
			continue
		case int16:
			updateField(m, k, int64(val))
			continue
		case int32:
			updateField(m, k, int64(val))
			continue
		case uint64:
			// InfluxDB does not support writing uint64
			if val < uint64(9223372036854775808) {
				updateField(m, k, int64(val))
			} else {
				updateField(m, k, int64(9223372036854775807))
			}
			continue
		case float32:
			updateField(m, k, float64(val))
			continue
		case float64:
			// NaNs are invalid values in influxdb, skip measurement
			if math.IsNaN(val) || math.IsInf(val, 0) {
				log.Printf("D! Measurement [%s] field [%s] has a NaN or Inf "+
					"field, skipping",
					m.Name(), k)
				m.RemoveField(k)
				continue
			}
		default:
			//do nothing
		}
	}

	GlobalMetricsGathered.Incr(1)
	return m
}
