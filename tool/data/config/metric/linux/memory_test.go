// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package linux

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
)

func TestMemory_ToMap(t *testing.T) {
	expectedKey := "mem"
	expectedValue := map[string]interface{}{"measurement": []string{"mem_used_percent"}}
	ctx := &runtime.Context{}
	conf := new(Memory)
	conf.Enable()
	key, value := conf.ToMap(ctx)
	assert.Equal(t, expectedKey, key)
	assert.Equal(t, expectedValue, value)
}
