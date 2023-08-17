// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package windows

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
)

func TestLogicalDisk_ToMap(t *testing.T) {
	expectedKey := "LogicalDisk"
	expectedValue := map[string]interface{}{"resources": []string{"*"}, "measurement": []string{"% Free Space"}}
	ctx := &runtime.Context{}
	conf := new(LogicalDisk)
	conf.Enable()
	key, value := conf.ToMap(ctx)
	assert.Equal(t, expectedKey, key)
	assert.Equal(t, expectedValue, value)
}
