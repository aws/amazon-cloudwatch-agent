// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ServiceDiscovery_InitPipelines(t *testing.T) {
	config := ServiceDiscoveryConfig{
		TargetCluster:       "test",
		TargetClusterRegion: "us-east-1",
	}
	p := &ServiceDiscovery{Config: &config}
	p.initClusterProcessorPipeline()

	assert.Equal(t, 8, len(p.clusterProcessors))
}

func Test_StartECSServiceDiscovery_NilConfig(t *testing.T) {
	var wg sync.WaitGroup
	p := &ServiceDiscovery{}
	wg.Add(1)
	StartECSServiceDiscovery(p, nil, &wg)
	assert.Equal(t, 0, len(p.clusterProcessors))
}

func Test_StartECSServiceDiscovery_NoServiceDiscovery(t *testing.T) {
	var wg sync.WaitGroup
	config := ServiceDiscoveryConfig{
		TargetCluster:       "test",
		TargetClusterRegion: "us-east-1",
	}
	p := &ServiceDiscovery{Config: &config}
	wg.Add(1)
	StartECSServiceDiscovery(p, nil, &wg)
	assert.Equal(t, 0, len(p.clusterProcessors))
}

func Test_StartECSServiceDiscovery_BadFrequency(t *testing.T) {
	var wg sync.WaitGroup
	config := ServiceDiscoveryConfig{
		TargetCluster:       "test",
		TargetClusterRegion: "us-east-1",
		Frequency:           "xyz",
		DockerLabel: &DockerLabelConfig{
			PortLabel: "TARGET_LABEL",
		},
	}
	p := &ServiceDiscovery{Config: &config}
	wg.Add(1)
	StartECSServiceDiscovery(p, nil, &wg)
	assert.Equal(t, 0, len(p.clusterProcessors))
}

func Test_StartECSServiceDiscovery_BadClusterConfig(t *testing.T) {
	var wg sync.WaitGroup
	config := ServiceDiscoveryConfig{
		Frequency: "1s",
		DockerLabel: &DockerLabelConfig{
			PortLabel: "TARGET_LABEL",
		},
	}
	p := &ServiceDiscovery{Config: &config}
	wg.Add(1)
	StartECSServiceDiscovery(p, nil, &wg)
	assert.Equal(t, 0, len(p.clusterProcessors))
}
