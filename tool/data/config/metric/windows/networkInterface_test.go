// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package windows

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
)

func TestNetworkInterface_ToMap(t *testing.T) {
	expectedKey := "Network Interface"
	expectedValue := map[string]interface{}{"resources": []string{"*"}, "measurement": []string{"Bytes Sent/sec", "Bytes Received/sec", "Packets Sent/sec", "Packets Received/sec"}}
	ctx := &runtime.Context{}
	conf := new(NetworkInterface)
	conf.Enable()
	key, value := conf.ToMap(ctx)
	assert.Equal(t, expectedKey, key)
	assert.Equal(t, expectedValue, value)
}
