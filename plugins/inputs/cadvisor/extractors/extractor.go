// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package extractors

import (
	"log"
	"time"

	cinfo "github.com/google/cadvisor/info/v1"

	"github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
)

const (
	containerNameLabel = "io.kubernetes.container.name"
	Metrics            = "Metrics"
	Dimensions         = "Dimensions"
	CleanInterval      = 5 * time.Minute
)

type MetricExtractor interface {
	HasValue(*cinfo.ContainerInfo) bool
	// GetValue normally applies to the following types:
	// containerinsightscommon.TypeContainer
	// containerinsightscommon.TypePod
	// containerinsightscommon.TypeNode
	// and ignores:
	// containerinsightscommon.TypeInfraContainer
	// The only exception is NetMetricExtractor because pod network metrics comes from infra container (i.e. pause).
	// See https://www.ianlewis.org/en/almighty-pause-container
	GetValue(info *cinfo.ContainerInfo, containerType string) []*CAdvisorMetric
	CleanUp(time.Time)
}

type CAdvisorMetric struct {
	cgroupPath string // source of the metric for debugging merge conflict
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
			log.Printf("D! metric being merged has conflict in fields, path src: %q, dest: %q", src.cgroupPath, c.cgroupPath)
			log.Printf("D! metric being merged has conflict in fields, src: %v, dest: %v", *src, *c)
			if c.tags[containerinsightscommon.Timestamp] < src.tags[containerinsightscommon.Timestamp] {
				continue
			}
		}
		c.fields[k] = v
	}
}
