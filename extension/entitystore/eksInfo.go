// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

import "go.uber.org/zap"

type ServiceEnvironment struct {
	ServiceName       string
	Environment       string
	ServiceNameSource string
}

type eksInfo struct {
	logger             *zap.Logger
	podToServiceEnvMap map[string]ServiceEnvironment
}

func newEKSInfo(logger *zap.Logger) *eksInfo {
	podToServiceEnvMap := make(map[string]ServiceEnvironment)
	return &eksInfo{
		logger:             logger,
		podToServiceEnvMap: podToServiceEnvMap,
	}
}

func (eks *eksInfo) AddPodServiceEnvironmentMapping(podName string, serviceName string, environmentName string, serviceNameSource string) {
	if eks.podToServiceEnvMap != nil {
		eks.podToServiceEnvMap[podName] = ServiceEnvironment{
			ServiceName:       serviceName,
			Environment:       environmentName,
			ServiceNameSource: serviceNameSource,
		}
	}
}

func (eks *eksInfo) GetPodServiceEnvironmentMapping() map[string]ServiceEnvironment {
	return eks.podToServiceEnvMap
}
