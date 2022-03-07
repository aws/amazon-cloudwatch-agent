// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import (
	"testing"

	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/stretchr/testify/assert"
)

func buildTestingTasksforTaskDef() []*DecoratedTask {
	taskDefArn1 := "arn:aws:ecs:us-east-2:1234567890:task-definition/prometheus-java-tomcat-fargate-awsvpc:1"
	taskDefArn2 := "arn:aws:ecs:us-east-2:1234567890:task-definition/prometheus-java-jar-ec2-bridge:2"
	taskDefArn3 := "arn:aws:ecs:us-east-2:1234567890:task-definition/prometheus-java-jar-ec2-bridge:3"
	taskDefArn4 := "arn:aws:ecs:us-east-2:1234567890:task-definition/prometheus-cwagent:12"
	return []*DecoratedTask{
		{
			TaskDefinition: &ecs.TaskDefinition{
				TaskDefinitionArn: &taskDefArn1,
			},
		},
		{
			TaskDefinition: &ecs.TaskDefinition{
				TaskDefinitionArn: &taskDefArn2,
			},
		},
		{
			TaskDefinition: &ecs.TaskDefinition{
				TaskDefinitionArn: &taskDefArn3,
			},
		},
		{
			TaskDefinition: &ecs.TaskDefinition{
				TaskDefinitionArn: &taskDefArn4,
			},
		},
		{
			TaskDefinition: &ecs.TaskDefinition{
				TaskDefinitionArn: nil,
			},
		},
	}
}

func Test_TaskDefinitionDiscoveryProcessor_EmptyConfig(t *testing.T) {
	p := NewTaskDefinitionDiscoveryProcessor(nil)
	assert.Equal(t, "TaskDefinitionDiscoveryProcessor", p.ProcessorName())
	taskList := buildTestingTasksforTaskDef()
	p.Process("test_ecs_cluster_name", taskList)

	for _, v := range taskList {
		assert.False(t, v.DockerLabelBased)
		assert.False(t, v.TaskDefinitionBased)
	}
}

func Test_TaskDefinitionDiscoveryProcessor_Normal(t *testing.T) {
	config := []*TaskDefinitionConfig{
		{TaskDefArnPattern: "^.*prometheus-java-jar-ec2-bridge:2$"},
		{TaskDefArnPattern: "^.*prometheus-java-tomcat-fargate-awsvpc:[1-9][0-9]*$"},
		{TaskDefArnPattern: "^.*task:[0-9]+$"},
	}

	taskList := buildTestingTasksforTaskDef()
	p := NewTaskDefinitionDiscoveryProcessor(config)
	assert.Equal(t, "TaskDefinitionDiscoveryProcessor", p.ProcessorName())
	p.Process("test_ecs_cluster_name", taskList)

	for _, v := range taskList {
		assert.False(t, v.DockerLabelBased)
	}
	assert.True(t, taskList[0].TaskDefinitionBased)
	assert.True(t, taskList[1].TaskDefinitionBased)
	assert.False(t, taskList[2].TaskDefinitionBased)
	assert.False(t, taskList[3].TaskDefinitionBased)
}

func Test_TaskDefinitionDiscoveryProcessor_ContainerName(t *testing.T) {
	config := []*TaskDefinitionConfig{
		{TaskDefArnPattern: "^.*prometheus-java-jar-ec2-bridge:2$",
			ContainerNamePattern: "^envoy$"},
	}

	taskDefArn := "arn:aws:ecs:us-east-2:1234567890:task-definition/prometheus-java-jar-ec2-bridge:2"
	containerNameEmpty := ""
	containerNameMismatch := "envoy_test"
	containerNameMatch := "envoy"
	tasks := []*DecoratedTask{
		{
			TaskDefinition: &ecs.TaskDefinition{
				TaskDefinitionArn: &taskDefArn,
			},
		},
		{
			TaskDefinition: &ecs.TaskDefinition{
				TaskDefinitionArn: &taskDefArn,
				ContainerDefinitions: []*ecs.ContainerDefinition{
					{},
				},
			},
		},
		{
			TaskDefinition: &ecs.TaskDefinition{
				TaskDefinitionArn: &taskDefArn,
				ContainerDefinitions: []*ecs.ContainerDefinition{
					{
						Name: &containerNameEmpty,
					},
				},
			},
		},
		{
			TaskDefinition: &ecs.TaskDefinition{
				TaskDefinitionArn: &taskDefArn,
				ContainerDefinitions: []*ecs.ContainerDefinition{
					{
						Name: &containerNameMismatch,
					},
				},
			},
		},
		{
			TaskDefinition: &ecs.TaskDefinition{
				TaskDefinitionArn: &taskDefArn,
				ContainerDefinitions: []*ecs.ContainerDefinition{
					{
						Name: &containerNameMatch,
					},
				},
			},
		},
	}

	p := NewTaskDefinitionDiscoveryProcessor(config)
	assert.Equal(t, "TaskDefinitionDiscoveryProcessor", p.ProcessorName())
	p.Process("test_ecs_cluster_name", tasks)

	assert.Equal(t, 5, len(tasks))
	assert.False(t, tasks[0].TaskDefinitionBased)
	assert.False(t, tasks[1].TaskDefinitionBased)
	assert.False(t, tasks[2].TaskDefinitionBased)
	assert.False(t, tasks[3].TaskDefinitionBased)
	assert.True(t, tasks[4].TaskDefinitionBased)
}
