// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsebsnvmereceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestNewFactory(t *testing.T) {
	c := NewFactory()
	assert.NotNil(t, c)
}

func TestCreateMetrics(t *testing.T) {
	metricsReceiver, _ := createMetricsReceiver(
		context.Background(),
		receivertest.NewNopSettings(),
		createDefaultConfig(),
		consumertest.NewNop(),
	)

	require.NotNil(t, metricsReceiver)
}
