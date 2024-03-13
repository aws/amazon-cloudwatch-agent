// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsneuron

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestAwsNeuronProcessor_ProcessMetrics(t *testing.T) {
	logger := zap.NewNop()
	config := &Config{}
	processor := newAwsNeuronProcessor(config, logger)
	processor.started = true

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	metrics := rm.ScopeMetrics().AppendEmpty().Metrics()
	metrics.AppendEmpty()

	ctx := context.Background()
	modifiedMd, err := processor.processMetrics(ctx, md)

	assert.NoError(t, err)
	assert.NotNil(t, modifiedMd)
	assert.Equal(t, 1, modifiedMd.ResourceMetrics().Len())
	assert.Equal(t, 1, modifiedMd.ResourceMetrics().At(0).ScopeMetrics().Len())
	assert.Equal(t, 1, modifiedMd.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())
}

func TestAwsNeuronProcessor_Start(t *testing.T) {
	logger := zap.NewNop()
	config := &Config{}
	processor := newAwsNeuronProcessor(config, logger)

	err := processor.Start(context.Background(), nil)

	assert.NoError(t, err)
	assert.True(t, processor.started)
}
