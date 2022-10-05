// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package adapter

import (
	"context"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/receiver/adapter/accumulator"
	"github.com/influxdata/telegraf/models"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
	"testing"
)

func Test_AdaptedReceiver_WithEmptyMetrics(t *testing.T) {
	t.Helper()

	as := assert.New(t)

	ri := models.NewRunningInput(&accumulator.TestRunningInput{}, &models.InputConfig{})
	adaptedReceiver := newAdaptedReceiver(ri, zap.NewNop())

	ctx := context.Background()
	err := adaptedReceiver.start(ctx, componenttest.NewNopHost())
	as.NoError(err)
	_, err = adaptedReceiver.scrape(ctx)
	as.NoError(err)
	err = adaptedReceiver.shutdown(ctx)
	as.NoError(err)

}
