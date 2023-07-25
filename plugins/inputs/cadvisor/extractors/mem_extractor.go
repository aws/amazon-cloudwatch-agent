// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package extractors

import (
	"time"

	cinfo "github.com/google/cadvisor/info/v1"

	. "github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"github.com/aws/amazon-cloudwatch-agent/internal/mapWithExpiry"
)

type MemMetricExtractor struct {
	preInfos *mapWithExpiry.MapWithExpiry
}

func (m *MemMetricExtractor) recordPreviousInfo(info *cinfo.ContainerInfo) {
	m.preInfos.Set(info.Name, info)
}

func (m *MemMetricExtractor) HasValue(info *cinfo.ContainerInfo) bool {
	return info.Spec.HasMemory
}

func (m *MemMetricExtractor) GetValue(info *cinfo.ContainerInfo, containerType string) []*CAdvisorMetric {
	var metrics []*CAdvisorMetric
	if containerType == TypeInfraContainer {
		return metrics
	}

	metric := newCadvisorMetric(containerType)
	metric.cgroupPath = info.Name
	curStats := GetStats(info)

	metric.fields[MetricName(containerType, MemUsage)] = curStats.Memory.Usage
	metric.fields[MetricName(containerType, MemCache)] = curStats.Memory.Cache
	metric.fields[MetricName(containerType, MemRss)] = curStats.Memory.RSS
	metric.fields[MetricName(containerType, MemMaxusage)] = curStats.Memory.MaxUsage
	metric.fields[MetricName(containerType, MemSwap)] = curStats.Memory.Swap
	metric.fields[MetricName(containerType, MemFailcnt)] = curStats.Memory.Failcnt
	metric.fields[MetricName(containerType, MemMappedfile)] = curStats.Memory.MappedFile
	metric.fields[MetricName(containerType, MemWorkingset)] = curStats.Memory.WorkingSet

	if preInfo, ok := m.preInfos.Get(info.Name); ok {
		preStats := GetStats(preInfo.(*cinfo.ContainerInfo))
		deltaCTimeInNano := curStats.Timestamp.Sub(preStats.Timestamp).Nanoseconds()
		if deltaCTimeInNano > MinTimeDiff {
			metric.fields[MetricName(containerType, MemPgfault)] = float64(curStats.Memory.ContainerData.Pgfault-preStats.Memory.ContainerData.Pgfault) / float64(deltaCTimeInNano) * float64(time.Second)
			metric.fields[MetricName(containerType, MemPgmajfault)] = float64(curStats.Memory.ContainerData.Pgmajfault-preStats.Memory.ContainerData.Pgmajfault) / float64(deltaCTimeInNano) * float64(time.Second)

			metric.fields[MetricName(containerType, MemHierarchicalPgfault)] = float64(curStats.Memory.HierarchicalData.Pgfault-preStats.Memory.HierarchicalData.Pgfault) / float64(deltaCTimeInNano) * float64(time.Second)
			metric.fields[MetricName(containerType, MemHierarchicalPgmajfault)] = float64(curStats.Memory.HierarchicalData.Pgmajfault-preStats.Memory.HierarchicalData.Pgmajfault) / float64(deltaCTimeInNano) * float64(time.Second)
		}
	}

	m.recordPreviousInfo(info)
	metrics = append(metrics, metric)
	return metrics
}

func (m *MemMetricExtractor) CleanUp(now time.Time) {
	m.preInfos.CleanUp(now)
}

func NewMemMetricExtractor() *MemMetricExtractor {
	return &MemMetricExtractor{
		preInfos: mapWithExpiry.NewMapWithExpiry(CleanInterval),
	}
}
