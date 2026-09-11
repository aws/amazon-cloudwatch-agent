// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/hashicorp/golang-lru/simplelru"
)

const (
	// ECS Service Quota: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service-quotas.html
	taskDefCacheSize = 2000
)

// Decorate the tasks with the ECS task definition
type TaskDefinitionProcessor struct {
	svcEcs *ecs.Client
	stats  *ProcessorStats

	taskDefCache *simplelru.LRU
}

func NewTaskDefinitionProcessor(ecsClient *ecs.Client, s *ProcessorStats) *TaskDefinitionProcessor {
	p := &TaskDefinitionProcessor{
		svcEcs: ecsClient,
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

func (p *TaskDefinitionProcessor) Process(ctx context.Context, _ string, taskList []*DecoratedTask) ([]*DecoratedTask, error) {
	defer func() {
		p.stats.AddStatsCount(LRUCacheSizeTaskDefinition, p.taskDefCache.Len())
	}()

	arn2Definition := make(map[string]*types.TaskDefinition)
	for _, t := range taskList {
		arn2Definition[aws.ToString(t.Task.TaskDefinitionArn)] = nil
	}

	for k := range arn2Definition {
		if k == "" {
			continue
		}

		var td *types.TaskDefinition
		if res, ok := p.taskDefCache.Get(k); ok {
			p.stats.AddStats(LRUCacheGetTaskDefinition)
			td = res.(*types.TaskDefinition)
		} else {
			resp, err := p.svcEcs.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{TaskDefinition: &k})
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
		v.TaskDefinition = arn2Definition[aws.ToString(v.Task.TaskDefinitionArn)]
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
