// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/stretchr/testify/assert"
)

func buildTestingTasksForServiceName() []*DecoratedTask {
	matchingID1 := "jghsdf3242"
	matchingID2 := "sdfdagfsdg"
	matchingID3 := "jfdnvufhsn"
	mismatchingID := "fnfjucnadz"
	containerNameMatch := "PatternMatch"
	containerNameMismatch := "InvalidPattern"
	return []*DecoratedTask{
		{
			TaskDefinition: &types.TaskDefinition{
				ContainerDefinitions: []types.ContainerDefinition{
					{
						Name: aws.String(containerNameMismatch),
					},
				},
			},
			Task: &types.Task{
				StartedBy: aws.String(matchingID1),
			},
		},
		{
			TaskDefinition: &types.TaskDefinition{
				ContainerDefinitions: []types.ContainerDefinition{
					{
						Name: aws.String(containerNameMatch),
					},
				},
			},
			Task: &types.Task{
				StartedBy: aws.String(matchingID2),
			},
		},
		{
			TaskDefinition: &types.TaskDefinition{
				ContainerDefinitions: []types.ContainerDefinition{
					{
						Name: aws.String(containerNameMismatch),
					},
				},
			},
			Task: &types.Task{
				StartedBy: aws.String(matchingID3),
			},
		},
		{
			TaskDefinition: &types.TaskDefinition{
				ContainerDefinitions: []types.ContainerDefinition{
					{
						Name: aws.String(containerNameMatch),
					},
				},
			},
			Task: &types.Task{
				StartedBy: aws.String(mismatchingID),
			},
		},
	}
}

func Test_ServiceNameDiscoveryProcessor_Normal(t *testing.T) {
	config := []*ServiceNameForTasksConfig{
		{ServiceNamePattern: "ServiceWithContainerNamePattern[1-9]+", ContainerNamePattern: "PatternMatch"},
		{ServiceNamePattern: "ServiceWithoutContainerNamePattern[1-9]+"},
	}
	var stats ProcessorStats
	taskList := buildTestingTasksForServiceName()
	mockSvc := &ecs.Client{}
	p := NewServiceEndpointDiscoveryProcessor(mockSvc, config, &stats)
	assert.Equal(t, "ServiceEndpointDiscoveryProcessor", p.ProcessorName())
	mismatchContainerMatchingID1 := "jghsdf3242"
	matchingContainerMatchingID := "sdfdagfsdg"
	mismatchContainerMatchingID2 := "jfdnvufhsn"
	idToServiceName := make(map[string]string)
	idToServiceName[mismatchContainerMatchingID1] = "ServiceWithContainerNamePattern1"
	idToServiceName[matchingContainerMatchingID] = "ServiceWithContainerNamePattern2"
	idToServiceName[mismatchContainerMatchingID2] = "ServiceWithoutContainerNamePattern3"
	p.processDecoratedTasks(taskList, idToServiceName)
	for _, v := range taskList {
		assert.False(t, v.DockerLabelBased)
		assert.False(t, v.TaskDefinitionBased)
	}
	assert.True(t, taskList[0].ServiceName == "")
	assert.False(t, taskList[1].ServiceName == "")
	assert.False(t, taskList[2].ServiceName == "")
	assert.True(t, taskList[3].ServiceName == "")
}
