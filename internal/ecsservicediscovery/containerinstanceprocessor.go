// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/hashicorp/golang-lru/simplelru"
)

const (
	// ECS Service Quota: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service-quotas.html
	ec2metadataCacheSize = 2000
	batchSize            = 100
)

// Add the Container instance metadata for ECS Clusters on Linux EC2 Instances
type ContainerInstanceProcessor struct {
	svcEc2 *ec2.Client
	svcEcs *ecs.Client
	stats  *ProcessorStats

	ec2MetaDataCache *simplelru.LRU
}

func NewContainerInstanceProcessor(ecsClient *ecs.Client, ec2Client *ec2.Client, s *ProcessorStats) *ContainerInstanceProcessor {
	p := &ContainerInstanceProcessor{
		svcEcs: ecsClient,
		svcEc2: ec2Client,
		stats:  s,
	}

	// initiate the container instance metadata LRU caching
	lru, err := simplelru.NewLRU(ec2metadataCacheSize, nil)
	if err != nil {
		log.Panicf("E! Initial container instance with caching failed because of %v", err)
	}
	p.ec2MetaDataCache = lru
	return p
}

func splitMapKeys(a map[string]*EC2MetaData, size int) [][]string {
	if size == 0 {
		log.Panic("splitMapKeys size cannot be zero.")
	}

	result := make([][]string, 0)
	v := make([]string, 0, size)
	for k := range a {
		if len(v) >= size {
			result = append(result, v)
			v = make([]string, 0, size)
		}
		v = append(v, k)
	}
	if len(v) > 0 {
		result = append(result, v)
	}
	return result
}

func (p *ContainerInstanceProcessor) handleContainerInstances(ctx context.Context, cluster string, batch []string, containerInstanceMap map[string]*EC2MetaData) error {
	ec2Id2containerInstanceIdMap := make(map[string]*string)
	input := &ecs.DescribeContainerInstancesInput{
		Cluster:            &cluster,
		ContainerInstances: batch,
	}
	resp, err := p.svcEcs.DescribeContainerInstances(ctx, input)
	p.stats.AddStats(AWSAPIDescribeContainerInstances)
	if err != nil {
		return newServiceDiscoveryError("Failed to DescribeContainerInstances", &err)
	}

	for _, f := range resp.Failures {
		log.Printf("E! DescribeContainerInstances Failure for %v, Reason: %v, Detail: %v \n", aws.ToString(f.Arn), aws.ToString(f.Reason), aws.ToString(f.Detail))
	}

	ec2Ids := make([]string, 0, batchSize)
	for _, ci := range resp.ContainerInstances {
		if ci.Ec2InstanceId != nil && ci.ContainerInstanceArn != nil {
			containerInstanceMap[aws.ToString(ci.ContainerInstanceArn)] = &EC2MetaData{
				ECInstanceId:        aws.ToString(ci.Ec2InstanceId),
				ContainerInstanceId: aws.ToString(ci.ContainerInstanceArn)}
			ec2Ids = append(ec2Ids, aws.ToString(ci.Ec2InstanceId))
			ec2Id2containerInstanceIdMap[aws.ToString(ci.Ec2InstanceId)] = ci.ContainerInstanceArn
		}
	}

	// Get the EC2 Instances
	ec2input := &ec2.DescribeInstancesInput{InstanceIds: ec2Ids}
	for {
		ec2resp, ec2err := p.svcEc2.DescribeInstances(ctx, ec2input)
		p.stats.AddStats(AWSCLIDescribeInstancesRequest)
		if ec2err != nil {
			return newServiceDiscoveryError("Failed to DescribeInstancesRequest", &ec2err)
		}

		for _, rsv := range ec2resp.Reservations {
			for _, ec2Instance := range rsv.Instances {
				ec2InstanceID := aws.ToString(ec2Instance.InstanceId)
				if ec2InstanceID == "" {
					continue
				}
				ciInstance, ok := ec2Id2containerInstanceIdMap[ec2InstanceID]
				if !ok {
					continue
				}
				containerInstanceMap[*ciInstance].PrivateIP = aws.ToString(ec2Instance.PrivateIpAddress)
				containerInstanceMap[*ciInstance].InstanceType = string(ec2Instance.InstanceType)
				containerInstanceMap[*ciInstance].SubnetId = aws.ToString(ec2Instance.SubnetId)
				containerInstanceMap[*ciInstance].VpcId = aws.ToString(ec2Instance.VpcId)
				p.ec2MetaDataCache.Add(*ciInstance, containerInstanceMap[*ciInstance])
			}
		}

		if ec2resp.NextToken == nil {
			break
		}
		ec2input.NextToken = ec2resp.NextToken
	}
	return nil
}

func (p *ContainerInstanceProcessor) Process(ctx context.Context, cluster string, taskList []*DecoratedTask) ([]*DecoratedTask, error) {
	defer func() {
		p.stats.AddStatsCount(LRUCacheSizeContainerInstance, p.ec2MetaDataCache.Len())
	}()
	containerInstanceMap := make(map[string]*EC2MetaData)
	for _, task := range taskList {
		if task.Task.LaunchType != types.LaunchTypeEc2 {
			continue
		}
		ciArn := aws.ToString(task.Task.ContainerInstanceArn)
		if ciArn != "" {
			if res, ok := p.ec2MetaDataCache.Get(ciArn); ok {
				p.stats.AddStats(LRUCacheGetEC2MetaData)
				task.EC2Info = res.(*EC2MetaData)
			} else {
				containerInstanceMap[ciArn] = nil
			}
		}
	}
	if len(containerInstanceMap) == 0 {
		return taskList, nil
	}
	batches := splitMapKeys(containerInstanceMap, batchSize)
	for _, batch := range batches {
		err := p.handleContainerInstances(ctx, cluster, batch, containerInstanceMap)
		if err != nil {
			return taskList, err
		}
	}
	for _, task := range taskList {
		if task.Task.ContainerInstanceArn != nil {
			if _, ok := containerInstanceMap[*task.Task.ContainerInstanceArn]; ok {
				task.EC2Info = containerInstanceMap[*task.Task.ContainerInstanceArn]
			}
		}
	}
	return taskList, nil
}

func (p *ContainerInstanceProcessor) ProcessorName() string {
	return "ContainerInstanceProcessor"
}
