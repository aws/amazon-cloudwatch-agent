// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package linux

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
)

func TestNetStat_ToMap(t *testing.T) {
	expectedKey := "netstat"
	expectedValue := map[string]interface{}{"measurement": []string{"tcp_established", "tcp_time_wait"}}
	ctx := &runtime.Context{}
	conf := new(NetStat)
	conf.Enable()
	key, value := conf.ToMap(ctx)
	assert.Equal(t, expectedKey, key)
	assert.Equal(t, expectedValue, value)
}
