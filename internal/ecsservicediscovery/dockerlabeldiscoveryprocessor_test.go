// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/stretchr/testify/assert"
)

func buildTestingTasksForDockerLabel() []*DecoratedTask {
	return []*DecoratedTask{
		{
			TaskDefinition: &types.TaskDefinition{
				ContainerDefinitions: []types.ContainerDefinition{
					{
						DockerLabels: map[string]string{"SELECTED_LABEL": "", "OTHER_LABELS": ""},
					},
				},
			},
		},
		{
			TaskDefinition: &types.TaskDefinition{
				ContainerDefinitions: []types.ContainerDefinition{
					{
						DockerLabels: map[string]string{"OTHER_LABELS": ""},
					},
				},
			},
		},
	}
}

func Test_DockerLabelDiscoveryProcessor_EmptyConfig(t *testing.T) {
	p := NewDockerLabelDiscoveryProcessor(nil)
	assert.Equal(t, "DockerLabelDiscoveryProcessor", p.ProcessorName())
	taskList := buildTestingTasksForDockerLabel()

	_, err := p.Process(t.Context(), "test_ecs_cluster_name", taskList)
	assert.NoError(t, err)

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
	taskList := buildTestingTasksForDockerLabel()
	p := NewDockerLabelDiscoveryProcessor(&config)
	assert.Equal(t, "DockerLabelDiscoveryProcessor", p.ProcessorName())
	_, err := p.Process(t.Context(), "test_ecs_cluster_name", taskList)
	assert.NoError(t, err)

	assert.True(t, taskList[0].DockerLabelBased)
	assert.False(t, taskList[1].DockerLabelBased)
	assert.False(t, taskList[0].TaskDefinitionBased)
	assert.False(t, taskList[1].TaskDefinitionBased)
}
