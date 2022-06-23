// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collectd

import (
	"testing"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/runtime"

	"github.com/stretchr/testify/assert"
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
