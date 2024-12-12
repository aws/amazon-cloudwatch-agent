// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import (
	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"github.com/stretchr/testify/mock"
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

func Test_StartECSServiceDiscovery_WithConfigurer(t *testing.T) {
	var wg sync.WaitGroup
	var requestHandlers []awsmiddleware.RequestHandler

	handler := new(awsmiddleware.MockHandler)
	handler.On("ID").Return("mock")
	handler.On("Position").Return(awsmiddleware.After)
	handler.On("HandleRequest", mock.Anything, mock.Anything)
	handler.On("HandleResponse", mock.Anything, mock.Anything)
	requestHandlers = append(requestHandlers, handler)
	middleware := new(awsmiddleware.MockMiddlewareExtension)
	middleware.On("Handlers").Return(
		requestHandlers,
		[]awsmiddleware.ResponseHandler{handler},
	)
	c := awsmiddleware.NewConfigurer(middleware.Handlers())

	config := ServiceDiscoveryConfig{
		TargetCluster:       "test",
		TargetClusterRegion: "us-east-1",
	}
	p := &ServiceDiscovery{Config: &config, Configurer: c}
	wg.Add(1)
	StartECSServiceDiscovery(p, nil, &wg)
	assert.Equal(t, 0, len(p.clusterProcessors))
}

func Test_StartECSServiceDiscovery_WithoutConfigurer(t *testing.T) {
	var wg sync.WaitGroup

	config := ServiceDiscoveryConfig{
		TargetCluster:       "test",
		TargetClusterRegion: "us-east-1",
	}
	p := &ServiceDiscovery{Config: &config, Configurer: nil}
	wg.Add(1)
	StartECSServiceDiscovery(p, nil, &wg)
	assert.Equal(t, 0, len(p.clusterProcessors))
}
