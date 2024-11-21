// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import (
	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"github.com/aws/aws-sdk-go/service/ecs"
	"log"
)

// Get all running tasks for the target cluster
type TaskProcessor struct {
	svcEcs     *ecs.ECS
	stats      *ProcessorStats
	Configurer *awsmiddleware.Configurer
}

// NewTaskProcessor creates a new TaskProcessor instance
func NewTaskProcessor(ecs *ecs.ECS, s *ProcessorStats, configurer *awsmiddleware.Configurer) *TaskProcessor {
	return &TaskProcessor{
		svcEcs:     ecs,
		stats:      s,
		Configurer: configurer,
	}
}

func (p *TaskProcessor) Process(cluster string, taskList []*DecoratedTask) ([]*DecoratedTask, error) {
	req := &ecs.ListTasksInput{Cluster: &cluster}
	for {
		if err := p.Configurer.Configure(awsmiddleware.SDKv1(&p.svcEcs.Handlers)); err != nil {
			log.Println("Failed to configure ecs client")
		} else {
			log.Println("Configured ecs client handlers!")
		}
		if &p.svcEcs.Handlers != nil {
			log.Println(p.svcEcs.Handlers)
		}

		listTaskResp, listTaskErr := p.svcEcs.ListTasks(req)

		p.stats.AddStats(AWSCLIListTasks)
		if listTaskErr != nil {
			return taskList, newServiceDiscoveryError("Failed to list task ARNs for "+cluster, &listTaskErr)
		}

		descTaskResp, descTaskErr := p.svcEcs.DescribeTasks(&ecs.DescribeTasksInput{Cluster: &cluster, Tasks: listTaskResp.TaskArns})
		p.stats.AddStats(AWSCLIDescribeTasks)
		if descTaskErr != nil {
			return taskList, newServiceDiscoveryError("Failed to describe ECS Tasks for "+cluster, &descTaskErr)
		}

		for _, f := range descTaskResp.Failures {
			log.Printf("E! DescribeTask Failure for %v, Reason: %v, Detail: %v \n", *f.Arn, *f.Reason, *f.Detail)
		}

		for i := 0; i < len(descTaskResp.Tasks); i++ {
			taskList = append(taskList, &DecoratedTask{Task: descTaskResp.Tasks[i], TaskDefinition: nil, EC2Info: nil})
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
