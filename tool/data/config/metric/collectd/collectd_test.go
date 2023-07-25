// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collectd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
)

func TestCollectD_ToMap(t *testing.T) {
	expectedKey := "collectd"
	expectedValue := map[string]interface{}{
		"metrics_aggregation_interval": 60,
	}
	ctx := new(runtime.Context)
	conf := new(CollectD)
	conf.Enable()
	key, value := conf.ToMap(ctx)
	assert.Equal(t, expectedKey, key)
	assert.Equal(t, expectedValue, value)
}
