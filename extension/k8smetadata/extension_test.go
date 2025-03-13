// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8smetadata

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/internal/k8sCommon/k8sclient"
)

func TestKubernetesMetadata_GetPodMetadata(t *testing.T) {
	esw := &k8sclient.EndpointSliceWatcher{
		IPToPodMetadata: &sync.Map{},
	}

	const testIP = "1.2.3.4"
	expected := k8sclient.PodMetadata{
		Workload:  "my-workload",
		Namespace: "my-namespace",
		Node:      "my-node",
	}
	esw.IPToPodMetadata.Store(testIP, expected)

	kMeta := &KubernetesMetadata{
		logger:               zap.NewNop(),
		endpointSliceWatcher: esw,
	}

	got := kMeta.GetPodMetadata(testIP)
	assert.Equal(t, expected, got, "GetPodMetadata should return the stored PodMetadata for %s", testIP)

	unknown := kMeta.GetPodMetadata("9.9.9.9")
	assert.Equal(t, k8sclient.PodMetadata{}, unknown, "GetPodMetadata should return empty if the IP is not present")
}

func TestKubernetesMetadata_GetPodMetadata_Incomplete(t *testing.T) {
	esw := &k8sclient.EndpointSliceWatcher{
		IPToPodMetadata: &sync.Map{},
	}

	const testIP = "2.2.2.2"
	expected := k8sclient.PodMetadata{
		Workload:  "incomplete-workload",
		Namespace: "",
		Node:      "",
	}
	esw.IPToPodMetadata.Store(testIP, expected)

	kMeta := &KubernetesMetadata{
		logger:               zap.NewNop(),
		endpointSliceWatcher: esw,
	}

	got := kMeta.GetPodMetadata(testIP)
	assert.Equal(t, expected, got, "GetPodMetadata should return the stored incomplete PodMetadata for IP %s", testIP)
}
