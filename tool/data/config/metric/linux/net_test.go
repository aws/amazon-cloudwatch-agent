// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package linux

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
)

func TestNet_ToMap(t *testing.T) {
	expectedKey := "net"
	expectedValue := map[string]interface{}{"resources": []string{"*"}, "measurement": []string{"bytes_sent", "bytes_recv", "packets_sent", "packets_recv"}}
	ctx := &runtime.Context{}
	conf := new(Net)
	conf.Enable()
	key, value := conf.ToMap(ctx)
	assert.Equal(t, expectedKey, key)
	assert.Equal(t, expectedValue, value)
}
