// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package linux

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
)

func TestSwap_ToMap(t *testing.T) {
	expectedKey := "swap"
	expectedValue := map[string]interface{}{"measurement": []string{"swap_used_percent"}}
	ctx := &runtime.Context{}
	conf := new(Swap)
	conf.Enable()
	key, value := conf.ToMap(ctx)
	assert.Equal(t, expectedKey, key)
	assert.Equal(t, expectedValue, value)
}
