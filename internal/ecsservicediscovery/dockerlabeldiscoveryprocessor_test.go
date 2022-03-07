// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import (
	"testing"

	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/stretchr/testify/assert"
)

func buildTestingTasksforDockerLabel() []*DecoratedTask {
	return []*DecoratedTask{
		{
			TaskDefinition: &ecs.TaskDefinition{
				ContainerDefinitions: []*ecs.ContainerDefinition{
					{
						DockerLabels: map[string]*string{"SELECTED_LABEL": nil, "OTHER_LABELS": nil},
					},
				},
			},
		},
		{
			TaskDefinition: &ecs.TaskDefinition{
				ContainerDefinitions: []*ecs.ContainerDefinition{
					{
						DockerLabels: map[string]*string{"OTHER_LABELS": nil},
					},
				},
			},
		},
	}
}

func Test_DockerLabelDiscoveryProcessor_EmptyConfig(t *testing.T) {

	p := NewDockerLabelDiscoveryProcessor(nil)
	assert.Equal(t, "DockerLabelDiscoveryProcessor", p.ProcessorName())
	taskList := buildTestingTasksforDockerLabel()

	p.Process("test_ecs_cluster_name", taskList)

	assert.False(t, taskList[0].DockerLabelBased)
	assert.False(t, taskList[1].DockerLabelBased)
	assert.False(t, taskList[0].TaskDefinitionBased)
	assert.False(t, taskList[1].TaskDefinitionBased)
}

func Test_DockerLabelDiscoveryProcessor_Normal(t *testing.T) {
	config := DockerLabelConfig{
		JobNameLabel:     "test_job_1",
		PortLabel:        "SELECTED_LABEL",
		MetricsPathLabel: "",
	}
	taskList := buildTestingTasksforDockerLabel()
	p := NewDockerLabelDiscoveryProcessor(&config)
	assert.Equal(t, "DockerLabelDiscoveryProcessor", p.ProcessorName())
	p.Process("test_ecs_cluster_name", taskList)

	assert.True(t, taskList[0].DockerLabelBased)
	assert.False(t, taskList[1].DockerLabelBased)
	assert.False(t, taskList[0].TaskDefinitionBased)
	assert.False(t, taskList[1].TaskDefinitionBased)
}
