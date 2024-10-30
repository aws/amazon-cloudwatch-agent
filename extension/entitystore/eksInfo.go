// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

import (
	"time"

	"github.com/jellydator/ttlcache/v3"
	"go.uber.org/zap"
)

const (
	ttlDuration = 5 * time.Minute

	// Agent server extension is mainly opened for FluentBit to
	// consume data and FluentBit only caches 256 pods in memory
	// so we will follow the same pattern
	maxPodAssociationMapCapacity = 256
)

type ServiceEnvironment struct {
	ServiceName       string
	Environment       string
	ServiceNameSource string
}

type eksInfo struct {
	logger             *zap.Logger
	podToServiceEnvMap *ttlcache.Cache[string, ServiceEnvironment]
}

func newEKSInfo(logger *zap.Logger) *eksInfo {
	return &eksInfo{
		logger: logger,
		podToServiceEnvMap: ttlcache.New[string, ServiceEnvironment](
			ttlcache.WithTTL[string, ServiceEnvironment](ttlDuration),
			ttlcache.WithCapacity[string, ServiceEnvironment](maxPodAssociationMapCapacity),
		),
	}
}

func (eks *eksInfo) AddPodServiceEnvironmentMapping(podName string, serviceName string, environmentName string, serviceNameSource string) {
	if eks.podToServiceEnvMap != nil {
		eks.podToServiceEnvMap.Set(podName, ServiceEnvironment{
			ServiceName:       serviceName,
			Environment:       environmentName,
			ServiceNameSource: serviceNameSource,
		}, ttlcache.DefaultTTL)
	}
}

func (eks *eksInfo) GetPodServiceEnvironmentMapping() *ttlcache.Cache[string, ServiceEnvironment] {
	return eks.podToServiceEnvMap
}
