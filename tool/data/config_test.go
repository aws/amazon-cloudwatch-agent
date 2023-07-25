// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package data

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

// more detailed internal content should be already tested in the individual struct unit tests
func TestConfig_ToMap(t *testing.T) {
	expectedKey := ""
	expectedValue := map[string]interface{}{
		"agent": map[string]interface{}{},
		"metrics": map[string]interface{}{
			"append_dimensions": map[string]interface{}{"InstanceType": "${aws:InstanceType}", "AutoScalingGroupName": "${aws:AutoScalingGroupName}", "ImageId": "${aws:ImageId}", "InstanceId": "${aws:InstanceId}"}},
		"logs": map[string]interface{}{}}
	conf := new(Config)
	ctx := &runtime.Context{
		OsParameter:          util.OsTypeLinux,
		WantEC2TagDimensions: true,
		IsOnPrem:             true,
	}
	conf.AgentConf()
	conf.MetricsConf()
	conf.LogsConf()
	key, value := conf.ToMap(ctx)
	assert.Equal(t, expectedKey, key)
	assert.Equal(t, expectedValue, value)

	conf = new(Config)
	ctx = &runtime.Context{
		OsParameter:          util.OsTypeDarwin,
		WantEC2TagDimensions: true,
		IsOnPrem:             true,
	}
	conf.AgentConf()
	conf.MetricsConf()
	conf.LogsConf()
	key, value = conf.ToMap(ctx)
	assert.Equal(t, expectedKey, key)
	assert.Equal(t, expectedValue, value)
}
