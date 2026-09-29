// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/stretchr/testify/assert"
)

func buildTestingTasksForTaskFilter() []*DecoratedTask {
	return []*DecoratedTask{
		{
			ServiceName:         "true",
			DockerLabelBased:    true,
			TaskDefinitionBased: true,
			TaskDefinition:      &types.TaskDefinition{},
		},
		{
			ServiceName:         "true",
			DockerLabelBased:    true,
			TaskDefinitionBased: false,
			TaskDefinition:      &types.TaskDefinition{},
		},
		{
			ServiceName:         "true",
			DockerLabelBased:    false,
			TaskDefinitionBased: true,
			TaskDefinition:      &types.TaskDefinition{},
		},
		{
			ServiceName:         "true",
			DockerLabelBased:    false,
			TaskDefinitionBased: false,
			TaskDefinition:      &types.TaskDefinition{},
		},
		{
			DockerLabelBased:    true,
			TaskDefinitionBased: true,
			TaskDefinition:      &types.TaskDefinition{},
		},
		{
			DockerLabelBased:    true,
			TaskDefinitionBased: false,
			TaskDefinition:      &types.TaskDefinition{},
		},
		{
			DockerLabelBased:    false,
			TaskDefinitionBased: true,
			TaskDefinition:      &types.TaskDefinition{},
		},
		{
			DockerLabelBased:    false,
			TaskDefinitionBased: false,
			TaskDefinition:      &types.TaskDefinition{},
		},
	}
}

func Test_NewTaskFilterProcessor_Normal(t *testing.T) {
	p := NewTaskFilterProcessor()
	assert.Equal(t, "TaskFilterProcessor", p.ProcessorName())
	taskList := buildTestingTasksForTaskFilter()
	taskList, _ = p.Process(t.Context(), "test_ecs_cluster_name", taskList)
	assert.Equal(t, 7, len(taskList))
}

func Test_NewTaskFilterProcessor_Empty(t *testing.T) {
	p := NewTaskFilterProcessor()
	taskList, _ := p.Process(t.Context(), "test_ecs_cluster_name", nil)
	assert.Equal(t, 0, len(taskList))
}
