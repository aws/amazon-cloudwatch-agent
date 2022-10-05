// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package accumulator

import (
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/models"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

const (
	defaultInstanceId      = "instance_id"
	defaultInstanceIdValue = "mock"
)

type TestRunningInput struct{}

func (t *TestRunningInput) Description() string                 { return "" }
func (t *TestRunningInput) SampleConfig() string                { return "" }
func (t *TestRunningInput) Gather(_ telegraf.Accumulator) error { return nil }

func generateExpectedAttributesWithMeasurement(measurement string) pcommon.Map {
	sampleAttributes := pcommon.NewMap()
	sampleAttributes.PutString(defaultInstanceId, defaultInstanceIdValue)
	sampleAttributes.PutString(measurementAttribute, measurement)
	return sampleAttributes
}

func newOtelAccumulatorWithTestRunningInputs(as *assert.Assertions) *otelAccumulator {

	ri := models.NewRunningInput(&TestRunningInput{}, &models.InputConfig{})
	as.NoError(ri.Config.Filter.Compile())

	return &otelAccumulator{
		input:     ri,
		logger:    zap.NewNop(),
		precision: time.Nanosecond,
		metrics:   pmetric.NewMetrics(),
	}
}
