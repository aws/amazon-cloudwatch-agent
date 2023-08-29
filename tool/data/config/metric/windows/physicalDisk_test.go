// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package windows

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
)

func TestPhysicalDisk_ToMap(t *testing.T) {
	expectedKey := "PhysicalDisk"
	expectedValue := map[string]interface{}{"resources": []string{"*"}, "measurement": []string{"% Disk Time", "Disk Write Bytes/sec", "Disk Read Bytes/sec", "Disk Writes/sec", "Disk Reads/sec"}}
	ctx := &runtime.Context{}
	conf := new(PhysicalDisk)
	conf.Enable()
	key, value := conf.ToMap(ctx)
	assert.Equal(t, expectedKey, key)
	assert.Equal(t, expectedValue, value)
}
