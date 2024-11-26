// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import (
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/hashicorp/golang-lru/simplelru"
)

const (
	// ECS Service Quota: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service-quotas.html
	taskDefCacheSize = 2000
)

// Decorate the tasks with the ECS task definition
type TaskDefinitionProcessor struct {
	svcEcs *ecs.ECS
	stats  *ProcessorStats

	taskDefCache *simplelru.LRU
}

func NewTaskDefinitionProcessor(ecs *ecs.ECS, s *ProcessorStats) *TaskDefinitionProcessor {
	p := &TaskDefinitionProcessor{
		svcEcs: ecs,
		stats:  s,
	}

	// initiate the caching
	lru, err := simplelru.NewLRU(taskDefCacheSize, nil)
	if err != nil {
		log.Panicf("E! Initial task definition with caching failed because of %v", err)
	}
	p.taskDefCache = lru
	return p
}

func (p *TaskDefinitionProcessor) Process(cluster string, taskList []*DecoratedTask) ([]*DecoratedTask, error) {
	defer func() {
		p.stats.AddStatsCount(LRUCacheSizeTaskDefinition, p.taskDefCache.Len())
	}()

	arn2Definition := make(map[string]*ecs.TaskDefinition)
	for _, t := range taskList {
		arn2Definition[aws.StringValue(t.Task.TaskDefinitionArn)] = nil
	}

	for k := range arn2Definition {
		if k == "" {
			continue
		}

		var td *ecs.TaskDefinition
		if res, ok := p.taskDefCache.Get(k); ok {
			p.stats.AddStats(LRUCacheGetTaskDefinition)
			td = res.(*ecs.TaskDefinition)
		} else {
			resp, err := p.svcEcs.DescribeTaskDefinition(&ecs.DescribeTaskDefinitionInput{TaskDefinition: &k})
			p.stats.AddStats(AWSCLIDescribeTaskDefinition)
			if err != nil {
				return taskList, newServiceDiscoveryError("Failed to describe task definition for "+k, &err)
			}
			p.taskDefCache.Add(k, resp.TaskDefinition)
			td = resp.TaskDefinition
		}
		arn2Definition[k] = td
	}

	for _, v := range taskList {
		v.TaskDefinition = arn2Definition[aws.StringValue(v.Task.TaskDefinitionArn)]
	}

	taskList = filterNilTaskDefinitionTasks(taskList)
	return taskList, nil
}

func filterNilTaskDefinitionTasks(taskList []*DecoratedTask) []*DecoratedTask {
	var filteredTasks []*DecoratedTask
	for _, v := range taskList {
		if v.TaskDefinition != nil {
			filteredTasks = append(filteredTasks, v)
		}
	}
	return filteredTasks
}

func (p *TaskDefinitionProcessor) ProcessorName() string {
	return "TaskDefinitionProcessor"
}
