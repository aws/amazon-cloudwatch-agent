// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import (
	"testing"

	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/stretchr/testify/assert"
)

func buildTestingTask() []*DecoratedTask {

	return []*DecoratedTask{
		{
			TaskDefinition: &ecs.TaskDefinition{},
		},
		{
			TaskDefinition: &ecs.TaskDefinition{},
		},
		{},
	}
}

func Test_filterNilTaskDefinitionTasks(t *testing.T) {
	tasks := filterNilTaskDefinitionTasks(buildTestingTask())
	assert.Equal(t, 2, len(tasks))
}
