// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheusadapter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestProcessMetricsForKueueMetrics(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	pap := newPrometheusAdapterProcessor(createDefaultConfig().(*Config), logger)

	assert.NotNil(t, pap)
}
