// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

// Get all running tasks for the target cluster
type TaskProcessor struct {
	svcEcs *ecs.Client
	stats  *ProcessorStats
}

func NewTaskProcessor(ecs *ecs.Client, s *ProcessorStats) *TaskProcessor {
	return &TaskProcessor{
		svcEcs: ecs,
		stats:  s,
	}
}

func (p *TaskProcessor) Process(ctx context.Context, cluster string, taskList []*DecoratedTask) ([]*DecoratedTask, error) {
	req := &ecs.ListTasksInput{Cluster: aws.String(cluster)}
	for {
		listTaskResp, listTaskErr := p.svcEcs.ListTasks(ctx, req)
		p.stats.AddStats(AWSCLIListTasks)
		if listTaskErr != nil {
			return taskList, newServiceDiscoveryError("Failed to list task ARNs for "+cluster, &listTaskErr)
		}

		descTaskResp, descTaskErr := p.svcEcs.DescribeTasks(ctx, &ecs.DescribeTasksInput{Cluster: aws.String(cluster), Tasks: listTaskResp.TaskArns})
		p.stats.AddStats(AWSCLIDescribeTasks)
		if descTaskErr != nil {
			return taskList, newServiceDiscoveryError("Failed to describe ECS Tasks for "+cluster, &descTaskErr)
		}

		for _, f := range descTaskResp.Failures {
			log.Printf("E! DescribeTask Failure for %v, Reason: %v, Detail: %v \n", aws.ToString(f.Arn), aws.ToString(f.Reason), aws.ToString(f.Detail))
		}

		for i := 0; i < len(descTaskResp.Tasks); i++ {
			taskList = append(taskList, &DecoratedTask{Task: &descTaskResp.Tasks[i], TaskDefinition: nil, EC2Info: nil})
		}

		if listTaskResp.NextToken == nil {
			break
		}
		req.NextToken = listTaskResp.NextToken
	}
	return taskList, nil
}

func (p *TaskProcessor) ProcessorName() string {
	return "TaskProcessor"
}
