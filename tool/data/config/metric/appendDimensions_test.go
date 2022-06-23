// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metric

import (
	"testing"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/runtime"

	"github.com/stretchr/testify/assert"
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
