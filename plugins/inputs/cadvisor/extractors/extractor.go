package extractors

import (
	"log"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	cinfo "github.com/google/cadvisor/info/v1"
)

const (
	containerNameLable = "io.kubernetes.container.name"
	infraContainerName = "POD"
	Metrics            = "Metrics"
	Dimensions         = "Dimensions"
	CleanInteval       = 5 * time.Minute
)

type MetricExtractor interface {
	HasValue(*cinfo.ContainerInfo) bool
	GetValue(*cinfo.ContainerInfo, string) []*CAdvisorMetric
	CleanUp(time.Time)
}

type CloudWatchMetrics struct {
	metrics    []string
	dimensions []string
}

type CAdvisorMetric struct {
	fields     map[string]interface{}
	tags       map[string]string
	metricType string
}

func newCadvisorMetric(mType string) *CAdvisorMetric {
	metric := &CAdvisorMetric{
		fields: make(map[string]interface{}),
		tags:   make(map[string]string),
	}
	metric.metricType = mType
	return metric
}

func (c *CAdvisorMetric) GetTags() map[string]string {
	return c.tags
}

func (c *CAdvisorMetric) GetFields() map[string]interface{} {
	return c.fields
}

func (c *CAdvisorMetric) GetAllTags() map[string]string {
	c.tags[containerinsightscommon.MetricType] = c.metricType
	return c.tags
}

func (c *CAdvisorMetric) GetMetricType() string {
	return c.metricType
}

func (c *CAdvisorMetric) AddTags(tags map[string]string) {
	for k, v := range tags {
		c.tags[k] = v
	}
}

func (c *CAdvisorMetric) Merge(src *CAdvisorMetric) {
	// If there is any conflict, keep the fields with earlier timestamp
	for k, v := range src.fields {
		if _, ok := c.fields[k]; ok {
			log.Printf("D! metric being merged has conflict in fields, src: %v, dest: %v \n", *src, *c)
			if c.tags[containerinsightscommon.Timestamp] < src.tags[containerinsightscommon.Timestamp] {
				continue
			}
		}
		c.fields[k] = v
	}
}
