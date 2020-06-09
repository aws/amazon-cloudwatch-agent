package extractors

import (
	. "github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	cinfo "github.com/google/cadvisor/info/v1"
	"log"
	"time"
)

type FileSystemMetricExtractor struct {
}

func (f *FileSystemMetricExtractor) HasValue(info *cinfo.ContainerInfo) bool {
	return info.Spec.HasFilesystem
}

func (f *FileSystemMetricExtractor) GetValue(info *cinfo.ContainerInfo, containerType string) []*CAdvisorMetric {
	var metrics []*CAdvisorMetric
	if containerType == TypePod || info.Spec.Labels[containerNameLable] == infraContainerName {
		return metrics
	}

	containerType = getFSMetricType(containerType)
	stats := GetStats(info)
	for _, v := range stats.Filesystem {
		metric := newCadvisorMetric(containerType)
		if v.Device == "" {
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
	return &FileSystemMetricExtractor{}
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
