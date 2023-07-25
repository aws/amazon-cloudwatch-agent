// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package linux

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
)

func TestCPU_ToMap(t *testing.T) {
	expectedKey := "cpu"
	expectedValue := map[string]interface{}{"resources": []string{"*"}, "totalcpu": true, "measurement": []string{"cpu_usage_idle", "cpu_usage_iowait", "cpu_usage_steal", "cpu_usage_guest", "cpu_usage_user", "cpu_usage_system"}}
	ctx := &runtime.Context{
		WantPerInstanceMetrics: true,
	}
	conf := new(CPU)
	conf.Enable()
	key, value := conf.ToMap(ctx)
	assert.Equal(t, expectedKey, key)
	assert.Equal(t, expectedValue, value)
}
