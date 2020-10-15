// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import (
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/stretchr/testify/assert"
	"testing"
)

func buildTestingTasksforTaskFilter() []*DecoratedTask {

	return []*DecoratedTask{
		&DecoratedTask{
			DockerLabelBased:    true,
			TaskDefinitionBased: true,
			TaskDefinition:      &ecs.TaskDefinition{},
		},
		&DecoratedTask{
			DockerLabelBased:    true,
			TaskDefinitionBased: false,
			TaskDefinition:      &ecs.TaskDefinition{},
		},
		&DecoratedTask{
			DockerLabelBased:    false,
			TaskDefinitionBased: true,
			TaskDefinition:      &ecs.TaskDefinition{},
		},
		&DecoratedTask{
			DockerLabelBased:    false,
			TaskDefinitionBased: false,
			TaskDefinition:      &ecs.TaskDefinition{},
		},
	}
}

func Test_NewTaskFilterProcessor_Normal(t *testing.T) {
	p := NewTaskFilterProcessor()
	assert.Equal(t, "TaskFilterProcessor", p.ProcessorName())
	taskList := buildTestingTasksforTaskFilter()
	taskList, _ = p.Process("test_ecs_cluster_name", taskList)
	assert.Equal(t, 3, len(taskList))
}

func Test_NewTaskFilterProcessor_Empty(t *testing.T) {
	p := NewTaskFilterProcessor()
	taskList, _ := p.Process("test_ecs_cluster_name", nil)
	assert.Equal(t, 0, len(taskList))
}
