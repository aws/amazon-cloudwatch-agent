// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/stretchr/testify/assert"
)

func buildTestingTask() []*DecoratedTask {

	return []*DecoratedTask{
		{
			TaskDefinition: &types.TaskDefinition{},
		},
		{
			TaskDefinition: &types.TaskDefinition{},
		},
		{},
	}
}

func Test_filterNilTaskDefinitionTasks(t *testing.T) {
	tasks := filterNilTaskDefinitionTasks(buildTestingTask())
	assert.Equal(t, 2, len(tasks))
}
