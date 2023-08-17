// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package windows

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
)

func TestProcessor_ToMap(t *testing.T) {
	expectedKey := "Processor"
	expectedValue := map[string]interface{}{"resources": []string{"_Total"}, "measurement": []string{"% Processor Time", "% User Time", "% Idle Time", "% Interrupt Time"}}
	ctx := &runtime.Context{}
	conf := new(Processor)
	conf.Enable()
	key, value := conf.ToMap(ctx)
	assert.Equal(t, expectedKey, key)
	assert.Equal(t, expectedValue, value)
}
