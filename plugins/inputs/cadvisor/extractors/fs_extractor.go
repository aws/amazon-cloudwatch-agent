// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package extractors

import (
	"log"
	"regexp"
	"time"

	. "github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	cinfo "github.com/google/cadvisor/info/v1"
)

const (
	allowList = "^tmpfs$|^/dev/|^overlay$"
)

type FileSystemMetricExtractor struct {
	allowListRegexP *regexp.Regexp
}

func (f *FileSystemMetricExtractor) HasValue(info *cinfo.ContainerInfo) bool {
	return info.Spec.HasFilesystem
}

func (f *FileSystemMetricExtractor) GetValue(info *cinfo.ContainerInfo, containerType string) []*CAdvisorMetric {
	var metrics []*CAdvisorMetric
	if containerType == TypePod || containerType == TypeInfraContainer {
		return metrics
	}

	containerType = getFSMetricType(containerType)
	stats := GetStats(info)

	for _, v := range stats.Filesystem {
		metric := newCadvisorMetric(containerType)
		if v.Device == "" {
			continue
		}
		if f.allowListRegexP != nil && !f.allowListRegexP.MatchString(v.Device) {
			continue
		}

		metric.tags[DiskDev] = v.Device
		metric.tags[FSType] = v.Type

		metric.fields[MetricName(containerType, FSUsage)] = v.Usage
		metric.fields[MetricName(containerType, FSCapacity)] = v.Limit
		metric.fields[MetricName(containerType, FSAvailable)] = v.Available

		if v.Limit != 0 {
			metric.fields[MetricName(containerType, FSUtilization)] = float64(v.Usage) / float64(v.Limit) * 100
		}

		if v.HasInodes {
			metric.fields[MetricName(containerType, FSInodes)] = v.Inodes
			metric.fields[MetricName(containerType, FSInodesfree)] = v.InodesFree
		}

		metrics = append(metrics, metric)
	}
	return metrics
}

func (f *FileSystemMetricExtractor) CleanUp(now time.Time) {
}

func NewFileSystemMetricExtractor() *FileSystemMetricExtractor {
	fse := &FileSystemMetricExtractor{}
	if p, err := regexp.Compile(allowList); err == nil {
		fse.allowListRegexP = p
	} else {
		log.Printf("E! NewFileSystemMetricExtractor set regex failed: %v", err)
	}

	return fse
}

func getFSMetricType(containerType string) string {
	metricType := ""
	switch containerType {
	case TypeNode:
		metricType = TypeNodeFS
	case TypeInstance:
		metricType = TypeInstanceFS
	case TypeContainer:
		metricType = TypeContainerFS
	default:
		log.Printf("W! fs_extractor: fs metric extractor is parsing unexpected containerType %s", containerType)
	}
	return metricType
}
