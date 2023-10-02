// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package accumulator

import (
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/models"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

const (
	defaultInstanceId      = "instance_id"
	defaultInstanceIdValue = "mock"
)

type TestRunningInput struct{}

var _ telegraf.Input = (*TestRunningInput)(nil)

func (t *TestRunningInput) Description() string                 { return "" }
func (t *TestRunningInput) SampleConfig() string                { return "" }
func (t *TestRunningInput) Gather(_ telegraf.Accumulator) error { return nil }

type TestServiceRunningInput struct{}

var _ telegraf.ServiceInput = (*TestServiceRunningInput)(nil)

func (t *TestServiceRunningInput) Description() string                 { return "" }
func (t *TestServiceRunningInput) SampleConfig() string                { return "" }
func (t *TestServiceRunningInput) Gather(_ telegraf.Accumulator) error { return nil }
func (t *TestServiceRunningInput) Start(_ telegraf.Accumulator) error  { return nil }
func (t *TestServiceRunningInput) Stop()                               {}

func generateExpectedAttributes() pcommon.Map {
	sampleAttributes := pcommon.NewMap()
	sampleAttributes.PutStr(defaultInstanceId, defaultInstanceIdValue)
	return sampleAttributes
}

func newOtelAccumulatorWithTestRunningInputs(as *assert.Assertions, consumer consumer.Metrics, isServiceInput bool) *otelAccumulator {
	return newOtelAccumulatorWithConfig(as, consumer, isServiceInput, &models.InputConfig{})
}

func newOtelAccumulatorWithConfig(as *assert.Assertions, consumer consumer.Metrics, isServiceInput bool, cfg *models.InputConfig) *otelAccumulator {
	var input telegraf.Input
	if isServiceInput {
		input = &TestServiceRunningInput{}
	} else {
		input = &TestRunningInput{}
	}
	ri := models.NewRunningInput(input, cfg)
	as.NoError(ri.Config.Filter.Compile())

	return &otelAccumulator{
		input:          ri,
		isServiceInput: isServiceInput,
		logger:         zap.NewNop(),
		precision:      time.Nanosecond,
		metrics:        pmetric.NewMetrics(),
		consumer:       consumer,
	}
}
