// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import (
	"log"
	"sort"
	"time"
)

const (
	AWSAPIDescribeContainerInstances = "AWSCLI_DescribeContainerInstances"
	AWSCLIDescribeInstancesRequest   = "AWSCLI_DescribeInstancesRequest"
	AWSCLIDescribeTaskDefinition     = "AWSCLI_DescribeTaskDefinition"
	AWSCLIListServices               = "AWSCLI_ListServices"
	AWSCLIListTasks                  = "AWSCLI_ListTasks"
	AWSCLIDescribeTasks              = "AWSCLI_DescribeTasks"
	LRUCacheGetEC2MetaData           = "LRUCache_Get_EC2MetaData"
	LRUCacheGetTaskDefinition        = "LRUCache_Get_TaskDefinition"
	LRUCacheSizeContainerInstance    = "LRUCache_Size_ContainerInstance"
	LRUCacheSizeTaskDefinition       = "LRUCache_Size_TaskDefinition"
	ExporterDiscoveredTargetCount    = "Exporter_DiscoveredTargetCount"
)

type ProcessorStats struct {
	stats     map[string]int
	startTime time.Time
}

func (sd *ProcessorStats) AddStats(name string) {
	sd.AddStatsCount(name, 1)
}

func (sd *ProcessorStats) GetStats(name string) int {
	if v, ok := sd.stats[name]; ok {
		return v
	}
	return 0
}

func (sd *ProcessorStats) AddStatsCount(name string, count int) {
	if sd.stats == nil {
		sd.ResetStats()
	}
	sd.stats[name] += count
}

func (sd *ProcessorStats) ResetStats() {
	sd.stats = make(map[string]int)
	sd.startTime = time.Now()
}

func (sd *ProcessorStats) ShowStats() {
	var keys []string
	for k := range sd.stats {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		log.Printf("D! ECS_SD_Stats: %v: %v\n", k, sd.stats[k])
	}
	log.Printf("D! ECS_SD_Stats: Latency: %v\n", time.Since(sd.startTime))
}
