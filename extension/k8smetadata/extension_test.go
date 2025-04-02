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

	got := kMeta.GetPodMetadataFromPodIP(testIP)
	assert.Equal(t, expected, got, "GetPodMetadata should return the stored PodMetadata for %s", testIP)

	unknown := kMeta.GetPodMetadataFromPodIP("9.9.9.9")
	assert.Equal(t, k8sclient.PodMetadata{}, unknown, "GetPodMetadata should return empty if the IP is not present")

	unknown = kMeta.GetPodMetadataFromPodIP("")
	assert.Equal(t, k8sclient.PodMetadata{}, unknown, "GetPodMetadata should return empty if the IP is empty")
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

	got := kMeta.GetPodMetadataFromPodIP(testIP)
	assert.Equal(t, expected, got, "GetPodMetadata should return the stored incomplete PodMetadata for IP %s", testIP)
}

func TestKubernetesMetadata_GetPodMetadataFromService(t *testing.T) {
	esw := &k8sclient.EndpointSliceWatcher{
		ServiceToPodMetadata: &sync.Map{},
	}

	const svcKey = "myservice@dev"
	expected := k8sclient.PodMetadata{
		Workload:  "my-workload",
		Namespace: "dev",
		Node:      "node-xyz",
	}
	esw.ServiceToPodMetadata.Store(svcKey, expected)

	kMeta := &KubernetesMetadata{
		logger:               zap.NewNop(),
		endpointSliceWatcher: esw,
	}

	got := kMeta.GetPodMetadataFromServiceAndNamespace(svcKey)
	assert.Equal(t, expected, got, "GetPodMetadataFromService should return the stored PodMetadata for %s", svcKey)

	unknown := kMeta.GetPodMetadataFromServiceAndNamespace("nosuchservice@dev")
	assert.Equal(t, k8sclient.PodMetadata{}, unknown, "Expected empty result for unknown service key")

	emptyVal := kMeta.GetPodMetadataFromServiceAndNamespace("")
	assert.Equal(t, k8sclient.PodMetadata{}, emptyVal, "Expected empty result for empty service key")
}

func TestKubernetesMetadata_GetServiceAndNamespaceFromClusterIP(t *testing.T) {
	mockSvcWatcher := &k8sclient.ServiceWatcher{
		IPToServiceAndNamespace: &sync.Map{},
	}

	const knownIP = "10.0.0.42"
	const knownSvcNS = "myservice@mynamespace"
	mockSvcWatcher.IPToServiceAndNamespace.Store(knownIP, knownSvcNS)

	mockESWatcher := &k8sclient.EndpointSliceWatcher{
		IPToPodMetadata:      &sync.Map{},
		ServiceToPodMetadata: &sync.Map{},
	}

	kMeta := &KubernetesMetadata{
		logger:               zap.NewNop(),
		endpointSliceWatcher: mockESWatcher,
		serviceWatcher:       mockSvcWatcher,
	}

	got := kMeta.GetServiceAndNamespaceFromClusterIP(knownIP)
	assert.Equal(t, knownSvcNS, got, "Expected to retrieve myservice@mynamespace for IP %s", knownIP)

	gotUnknown := kMeta.GetServiceAndNamespaceFromClusterIP("10.0.0.99")
	assert.Equal(t, "", gotUnknown, "Expected empty string for unknown cluster IP")

	gotEmpty := kMeta.GetServiceAndNamespaceFromClusterIP("")
	assert.Equal(t, "", gotEmpty, "Expected empty string when IP is empty")
}
