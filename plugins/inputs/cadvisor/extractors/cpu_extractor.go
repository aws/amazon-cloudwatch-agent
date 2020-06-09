package extractors

import (
	"time"

	. "github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"github.com/aws/amazon-cloudwatch-agent/internal/mapWithExpiry"
	cInfo "github.com/google/cadvisor/info/v1"
)

const (
	decimalToMillicores = 1000
)

type CpuMetricExtractor struct {
	preInfos *mapWithExpiry.MapWithExpiry
}

func (c *CpuMetricExtractor) HasValue(info *cInfo.ContainerInfo) bool {
	return info.Spec.HasCpu
}

func (c *CpuMetricExtractor) recordPreviousInfo(info *cInfo.ContainerInfo) {
	c.preInfos.Set(info.Name, info)
}

func (c *CpuMetricExtractor) GetValue(info *cInfo.ContainerInfo, containerType string) []*CAdvisorMetric {
	var metrics []*CAdvisorMetric
	if info.Spec.Labels[containerNameLable] == infraContainerName {
		return metrics
	}

	if preInfo, ok := c.preInfos.Get(info.Name); ok {
		// When there is more than one stats point, always use the last one
		curStats := GetStats(info)
		preStats := GetStats(preInfo.(*cInfo.ContainerInfo))
		deltaCTimeInNano := curStats.Timestamp.Sub(preStats.Timestamp).Nanoseconds()

		if deltaCTimeInNano > MinTimeDiff {
			metric := newCadvisorMetric(containerType)

			metric.fields[MetricName(containerType, CpuTotal)] = float64(curStats.Cpu.Usage.Total-preStats.Cpu.Usage.Total) / float64(deltaCTimeInNano) * decimalToMillicores
			metric.fields[MetricName(containerType, CpuUser)] = float64(curStats.Cpu.Usage.User-preStats.Cpu.Usage.User) / float64(deltaCTimeInNano) * decimalToMillicores
			metric.fields[MetricName(containerType, CpuSystem)] = float64(curStats.Cpu.Usage.System-preStats.Cpu.Usage.System) / float64(deltaCTimeInNano) * decimalToMillicores

			metrics = append(metrics, metric)
		}
	}
	c.recordPreviousInfo(info)
	return metrics
}

func (c *CpuMetricExtractor) CleanUp(now time.Time) {
	c.preInfos.CleanUp(now)
}

func NewCpuMetricExtractor() *CpuMetricExtractor {
	return &CpuMetricExtractor{
		preInfos: mapWithExpiry.NewMapWithExpiry(CleanInteval),
	}
}
