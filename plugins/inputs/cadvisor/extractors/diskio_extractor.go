// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package extractors

import (
	"fmt"
	"log"
	"strings"
	"time"

	. "github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"github.com/aws/amazon-cloudwatch-agent/internal/mapWithExpiry"
	cinfo "github.com/google/cadvisor/info/v1"
)

type DiskIOMetricExtractor struct {
	preInfos *mapWithExpiry.MapWithExpiry
}

func (d *DiskIOMetricExtractor) recordPreviousInfo(info *cinfo.ContainerInfo) {
	d.preInfos.Set(info.Name, info)
}

func (d *DiskIOMetricExtractor) HasValue(info *cinfo.ContainerInfo) bool {
	return info.Spec.HasDiskIo
}

func (d *DiskIOMetricExtractor) GetValue(info *cinfo.ContainerInfo, containerType string) []*CAdvisorMetric {
	var metrics []*CAdvisorMetric
	if containerType != TypeNode && containerType != TypeInstance {
		return metrics
	}

	if preInfo, ok := d.preInfos.Get(info.Name); ok {
		curStats := GetStats(info)
		preStats := GetStats(preInfo.(*cinfo.ContainerInfo))
		deltaCTimeInNano := curStats.Timestamp.Sub(preStats.Timestamp).Nanoseconds()
		if deltaCTimeInNano > MinTimeDiff {
			metrics = append(metrics, extractIoMetrics(curStats.DiskIo.IoServiceBytes, preStats.DiskIo.IoServiceBytes, DiskIOServiceBytesPrefix, deltaCTimeInNano, containerType)...)
			metrics = append(metrics, extractIoMetrics(curStats.DiskIo.IoServiced, preStats.DiskIo.IoServiced, DiskIOServicedPrefix, deltaCTimeInNano, containerType)...)
		}
	}
	d.recordPreviousInfo(info)
	return metrics
}

func extractIoMetrics(curStatsSet []cinfo.PerDiskStats, preStatsSet []cinfo.PerDiskStats, namePrefix string, deltaCTimeInNanoSec int64, containerType string) []*CAdvisorMetric {
	var metrics []*CAdvisorMetric
	expectedKey := []string{DiskIOAsync, DiskIOSync, DiskIORead, DiskIOWrite, DiskIOTotal}
	for _, cur := range curStatsSet {
		curDevName := devName(cur)
		for _, pre := range preStatsSet {
			preDevName := devName(pre)
			if curDevName == preDevName {
				metric := newCadvisorMetric(getDiskIOMetricType(containerType))
				metric.tags[DiskDev] = curDevName
				for _, key := range expectedKey {
					curVal, curOk := cur.Stats[key]
					preVal, preOk := pre.Stats[key]
					if curOk && preOk {
						mname := MetricName(containerType, ioMetricName(namePrefix, key))
						metric.fields[mname] = float64(curVal-preVal) / float64(deltaCTimeInNanoSec) * float64(time.Second)
					}
				}
				if len(metric.fields) > 0 {
					metrics = append(metrics, metric)
				}
				break
			}
		}
	}
	return metrics
}

func ioMetricName(prefix, key string) string {
	return prefix + strings.ToLower(key)
}

func devName(dStats cinfo.PerDiskStats) string {
	devName := dStats.Device
	if devName == "" {
		devName = fmt.Sprintf("%d:%d", dStats.Major, dStats.Minor)
	}
	return devName
}

func (d *DiskIOMetricExtractor) CleanUp(now time.Time) {
	d.preInfos.CleanUp(now)
}

func NewDiskIOMetricExtractor() *DiskIOMetricExtractor {
	return &DiskIOMetricExtractor{
		preInfos: mapWithExpiry.NewMapWithExpiry(CleanInterval),
	}
}

func getDiskIOMetricType(containerType string) string {
	metricType := ""
	switch containerType {
	case TypeNode:
		metricType = TypeNodeDiskIO
	case TypeInstance:
		metricType = TypeInstanceDiskIO
	case TypeContainer:
		metricType = TypeContainerDiskIO
	default:
		log.Printf("W! diskio_extractor: diskIO metric extractor is parsing unexpected containerType %s", containerType)
	}
	return metricType
}
