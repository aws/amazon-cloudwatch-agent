// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metric

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
)

func TestAggregationDimension_ToMap(t *testing.T) {
	expectedKey := "aggregation_dimensions"
	expectedValue := [][]string{{"InstanceId"}}
	ctx := &runtime.Context{}
	conf := new(AggregationDimensions)
	key, value := conf.ToMap(ctx)
	assert.Equal(t, expectedKey, key)
	assert.Equal(t, expectedValue, value)
}
