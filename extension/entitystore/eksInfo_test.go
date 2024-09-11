// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestAddPodServiceEnvironmentMapping(t *testing.T) {
	tests := []struct {
		name    string
		want    map[string]ServiceEnvironment
		podName string
		service string
		env     string
		mapNil  bool
	}{
		{
			name: "AddPodWithServiceMapping",
			want: map[string]ServiceEnvironment{
				"test-pod": {
					ServiceName: "test-service",
				},
			},
			podName: "test-pod",
			service: "test-service",
		},
		{
			name: "AddPodWithServiceEnvMapping",
			want: map[string]ServiceEnvironment{
				"test-pod": {
					ServiceName: "test-service",
					Environment: "test-env",
				},
			},
			podName: "test-pod",
			service: "test-service",
			env:     "test-env",
		},
		{
			name:   "AddWhenPodToServiceMapIsNil",
			mapNil: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, _ := zap.NewDevelopment()
			ei := newEKSInfo(logger)
			if tt.mapNil {
				ei.podToServiceEnvMap = nil
			}
			ei.AddPodServiceEnvironmentMapping(tt.podName, tt.service, tt.env)
			assert.Equal(t, tt.want, ei.podToServiceEnvMap)
		})
	}
}

func TestGetPodServiceEnvironmentMapping(t *testing.T) {
	tests := []struct {
		name   string
		want   map[string]ServiceEnvironment
		addMap bool
	}{
		{
			name: "GetPodWithServiceEnvMapping",
			want: map[string]ServiceEnvironment{
				"test-pod": {
					ServiceName: "test-service",
					Environment: "test-env",
				},
			},
			addMap: true,
		},
		{
			name: "GetWhenPodToServiceMapIsEmpty",
			want: map[string]ServiceEnvironment{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, _ := zap.NewDevelopment()
			ei := newEKSInfo(logger)
			if tt.addMap {
				ei.AddPodServiceEnvironmentMapping("test-pod", "test-service", "test-env")
			}
			assert.Equal(t, tt.want, ei.GetPodServiceEnvironmentMapping())
		})
	}
}
