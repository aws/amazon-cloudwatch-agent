// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/runtime"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/util"
)

func TestAgent_ToMap(t *testing.T) {
	expectedKey := "agent"
	expectedValue := map[string]interface{}{util.MapKeyMetricsCollectionInterval: 10}
	ctx := &runtime.Context{MetricsCollectionInterval: 10}
	conf := new(AgentConfig)
	conf.Runasuser = ""
	key, value := conf.ToMap(ctx)
	assert.Equal(t, expectedKey, key)
	assert.Equal(t, expectedValue, value)

	runAsUser := "cwagent"
	expectedValue = map[string]interface{}{util.MapKeyMetricsCollectionInterval: 10, RUNASUSER: runAsUser}
	conf.Runasuser = runAsUser
	key, value = conf.ToMap(ctx)
	assert.Equal(t, expectedKey, key)
	assert.Equal(t, expectedValue, value)
}
