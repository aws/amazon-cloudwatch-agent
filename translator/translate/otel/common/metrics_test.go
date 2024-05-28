// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/internal/metric"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/testutil"
)

func TestGetRollupDimensions(t *testing.T) {
	jsonCfg := testutil.GetJson(t, filepath.Join("testdata", "config.json"))
	conf := confmap.NewFromStringMap(jsonCfg)
	assert.Equal(t, [][]string{{"ImageId"}, {"InstanceId", "InstanceType"}, {"d1"}, {}}, GetRollupDimensions(conf))
}

func TestGetDropOriginalMetrics(t *testing.T) {
	jsonCfg := testutil.GetJson(t, filepath.Join("testdata", "config.json"))
	conf := confmap.NewFromStringMap(jsonCfg)
	assert.Equal(t, map[string]bool{
		"CPU_USAGE_IDLE": true,
		metric.DecorateMetricName("cpu", "time_active"): true,
	}, GetDropOriginalMetrics(conf))
}
