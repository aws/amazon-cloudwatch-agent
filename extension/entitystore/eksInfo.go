// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

import (
	"time"

	"github.com/jellydator/ttlcache/v3"
	"go.uber.org/zap"
)

const ttlDuration = 5 * time.Minute

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
