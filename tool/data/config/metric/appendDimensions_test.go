// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metric

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
)

func TestGlobalDimension_ToMap(t *testing.T) {
	expectedKey := "append_dimensions"
	expectedValue := map[string]interface{}{"ImageId": "${aws:ImageId}", "InstanceId": "${aws:InstanceId}", "InstanceType": "${aws:InstanceType}", "AutoScalingGroupName": "${aws:AutoScalingGroupName}"}
	ctx := &runtime.Context{}
	conf := new(AppendDimensions)
	key, value := conf.ToMap(ctx)
	assert.Equal(t, expectedKey, key)
	assert.Equal(t, expectedValue, value)
}
