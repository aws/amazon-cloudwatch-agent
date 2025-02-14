// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

import (
	"strconv"
	"testing"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestAddPodServiceEnvironmentMapping(t *testing.T) {
	tests := []struct {
		name              string
		want              *ttlcache.Cache[string, ServiceEnvironment]
		podName           string
		service           string
		env               string
		serviceNameSource string
		mapNil            bool
	}{
		{
			name: "AddPodWithServiceMapping",
			want: setupTTLCacheForTesting(map[string]ServiceEnvironment{
				"test-pod": {
					ServiceName: "test-service",
				},
			}, ttlDuration),
			podName: "test-pod",
			service: "test-service",
		},
		{
			name: "AddPodWithServiceEnvMapping",
			want: setupTTLCacheForTesting(map[string]ServiceEnvironment{
				"test-pod": {
					ServiceName: "test-service",
					Environment: "test-env",
				},
			}, ttlDuration),
			podName: "test-pod",
			service: "test-service",
			env:     "test-env",
		},
		{
			name: "AddPodWithServiceEnvMapping",
			want: setupTTLCacheForTesting(map[string]ServiceEnvironment{
				"test-pod": {
					ServiceName:       "test-service",
					Environment:       "test-env",
					ServiceNameSource: ServiceNameSourceInstrumentation,
				},
			}, ttlDuration),
			podName:           "test-pod",
			service:           "test-service",
			env:               "test-env",
			serviceNameSource: "Instrumentation",
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
			ei.AddPodServiceEnvironmentMapping(tt.podName, tt.service, tt.env, tt.serviceNameSource)
			if tt.mapNil {
				assert.Nil(t, ei.podToServiceEnvMap)
			} else {
				for pod, se := range tt.want.Items() {
					assert.Equal(t, se.Value(), ei.GetPodServiceEnvironmentMapping().Get(pod).Value())
				}
				assert.Equal(t, tt.want.Len(), ei.GetPodServiceEnvironmentMapping().Len())
			}
		})
	}
}

func TestAddPodServiceEnvironmentMapping_TtlRefresh(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ei := newEKSInfo(logger)

	//adds new pod to service environment mapping
	ei.AddPodServiceEnvironmentMapping("test-pod", "test-service", "test-environment", "Instrumentation")
	assert.Equal(t, 1, ei.podToServiceEnvMap.Len())
	expiration := ei.podToServiceEnvMap.Get("test-pod").ExpiresAt()

	//sleep for 1 second to simulate ttl refresh
	time.Sleep(1 * time.Second)

	// simulate adding the same pod to service environment mapping
	ei.AddPodServiceEnvironmentMapping("test-pod", "test-service", "test-environment", "Instrumentation")
	newExpiration := ei.podToServiceEnvMap.Get("test-pod").ExpiresAt()

	// assert that the expiration time is updated
	assert.True(t, newExpiration.After(expiration))
	assert.Equal(t, 1, ei.podToServiceEnvMap.Len())
}

func TestAddPodServiceEnvironmentMapping_MaxCapacity(t *testing.T) {
	logger := zap.NewNop()
	ei := newEKSInfo(logger)

	//adds new pod to service environment mapping
	for i := 0; i < 300; i++ {
		ei.AddPodServiceEnvironmentMapping("test-pod-"+strconv.Itoa(i), "test-service", "test-environment", "Instrumentation")
	}
	assert.Equal(t, maxPodAssociationMapCapacity, ei.podToServiceEnvMap.Len())
	itemIndex := 299
	ei.podToServiceEnvMap.Range(func(item *ttlcache.Item[string, ServiceEnvironment]) bool {
		// Check if the item's value equals the target string
		assert.Equal(t, item.Key(), "test-pod-"+strconv.Itoa(itemIndex))
		itemIndex--
		return true
	})
}

func TestGetPodServiceEnvironmentMapping(t *testing.T) {
	tests := []struct {
		name   string
		want   *ttlcache.Cache[string, ServiceEnvironment]
		addMap bool
	}{
		{
			name: "GetPodWithServiceEnvMapping",
			want: setupTTLCacheForTesting(map[string]ServiceEnvironment{
				"test-pod": {
					ServiceName:       "test-service",
					Environment:       "test-env",
					ServiceNameSource: "test-service-name-source",
				},
			}, ttlDuration),
			addMap: true,
		},
		{
			name: "GetWhenPodToServiceMapIsEmpty",
			want: setupTTLCacheForTesting(map[string]ServiceEnvironment{}, ttlDuration),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, _ := zap.NewDevelopment()
			ei := newEKSInfo(logger)
			if tt.addMap {
				ei.AddPodServiceEnvironmentMapping("test-pod", "test-service", "test-env", "test-service-name-source")
			}
			for pod, se := range tt.want.Items() {
				assert.Equal(t, se.Value(), ei.GetPodServiceEnvironmentMapping().Get(pod).Value())
			}
			assert.Equal(t, tt.want.Len(), ei.GetPodServiceEnvironmentMapping().Len())
		})
	}
}

func TestTTLServicePodEnvironmentMapping(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ei := newEKSInfo(logger)

	ei.podToServiceEnvMap = setupTTLCacheForTesting(map[string]ServiceEnvironment{
		"pod": {
			ServiceName: "service",
			Environment: "environment",
		},
	}, 500*time.Millisecond)
	// this assertion relies on the speed of your computer to get this done before
	// the cache evicts the item based on the TTL
	assert.Equal(t, 1, ei.podToServiceEnvMap.Len())

	//starting the ttl cache like we do in code. This will automatically evict expired pods.
	go ei.podToServiceEnvMap.Start()
	defer ei.podToServiceEnvMap.Stop()

	//sleep for 1 second to simulate ttl refresh
	time.Sleep(1 * time.Second)

	//stops the ttl cache.
	assert.Equal(t, 0, ei.podToServiceEnvMap.Len())
}

func setupTTLCacheForTesting(podToServiceMap map[string]ServiceEnvironment, ttlDuration time.Duration) *ttlcache.Cache[string, ServiceEnvironment] {
	cache := ttlcache.New[string, ServiceEnvironment](ttlcache.WithTTL[string, ServiceEnvironment](ttlDuration))
	for pod, serviceEnv := range podToServiceMap {
		cache.Set(pod, serviceEnv, ttlcache.DefaultTTL)
	}
	return cache
}
