// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package adapter

import (
	"context"
	"testing"

	"github.com/influxdata/telegraf/models"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/receiver/adapter/accumulator"
)

func Test_AdaptedReceiver_WithEmptyMetrics(t *testing.T) {
	t.Helper()

	as := assert.New(t)

	ctx := context.Background()
	ri := models.NewRunningInput(&accumulator.TestRunningInput{}, &models.InputConfig{})
	adaptedReceiver := newAdaptedReceiver(ri, ctx, nil, zap.NewNop())

	err := adaptedReceiver.start(ctx, componenttest.NewNopHost())
	as.NoError(err)
	_, err = adaptedReceiver.scrape(ctx)
	as.NoError(err)
	err = adaptedReceiver.shutdown(ctx)
	as.NoError(err)
}

func Test_AdaptedReceiver_WithEmptyMetrics_ServiceInput(t *testing.T) {
	t.Helper()

	as := assert.New(t)

	ctx := context.Background()
	ri := models.NewRunningInput(&accumulator.TestServiceRunningInput{}, &models.InputConfig{})
	adaptedReceiver := newAdaptedReceiver(ri, ctx, &consumertest.MetricsSink{}, zap.NewNop())

	err := adaptedReceiver.start(ctx, componenttest.NewNopHost())
	as.NoError(err)
	_, err = adaptedReceiver.scrape(ctx)
	as.NoError(err)
	err = adaptedReceiver.shutdown(ctx)
	as.NoError(err)
}
