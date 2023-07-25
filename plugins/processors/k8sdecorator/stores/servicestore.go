// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package stores

import (
	"log"
	"sync"
	"time"

	"github.com/influxdata/telegraf"

	. "github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"github.com/aws/amazon-cloudwatch-agent/internal/k8sCommon/k8sclient"
)

const (
	refreshIntervalService = 10 //10s
)

type ServiceStore struct {
	podKeyToServiceNamesMap map[string][]string
	sync.Mutex
	lastRefreshed time.Time
}

func NewServiceStore() *ServiceStore {
	serviceStore := &ServiceStore{
		podKeyToServiceNamesMap: make(map[string][]string),
	}
	return serviceStore
}

func (s *ServiceStore) RefreshTick() {
	now := time.Now()
	if now.Sub(s.lastRefreshed).Seconds() >= refreshIntervalService {
		s.refresh()
		s.lastRefreshed = now
	}
}

// service info is not mandatory
func (s *ServiceStore) Decorate(metric telegraf.Metric, kubernetesBlob map[string]interface{}) bool {
	tags := metric.Tags()
	if _, ok := tags[K8sPodNameKey]; ok {
		podKey := createPodKeyFromMetric(tags)
		if podKey == "" {
			log.Printf("E! podKey is unavailable when decorating service.")
			return false
		}
		if serviceList, ok := s.podKeyToServiceNamesMap[podKey]; ok {
			if len(serviceList) > 0 {
				addServiceNameTag(metric, serviceList)
			}
		}
	}
	return true
}

func (s *ServiceStore) refresh() {
	s.podKeyToServiceNamesMap = k8sclient.Get().Ep.PodKeyToServiceNames()
}

func addServiceNameTag(metric telegraf.Metric, serviceNames []string) {
	// TODO handle serviceNames len is larger than 1. We need to duplicate the metric object
	metric.AddTag(TypeService, serviceNames[0])
}
