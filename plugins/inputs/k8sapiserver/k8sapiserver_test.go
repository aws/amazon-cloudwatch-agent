// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sapiserver

import (
	"log"
	"os"
	"testing"

	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"github.com/aws/amazon-cloudwatch-agent/internal/k8sCommon/k8sclient"
)

var mockClient = new(MockClient)

var mockK8sClient = &k8sclient.K8sClient{
	Pod:  mockClient,
	Node: mockClient,
	Ep:   mockClient,
}

func mockGet() *k8sclient.K8sClient {
	return mockK8sClient
}

type MockClient struct {
	k8sclient.PodClient
	k8sclient.NodeClient
	k8sclient.EpClient

	mock.Mock
}

// k8sclient.PodClient
func (client *MockClient) NamespaceToRunningPodNum() map[string]int {
	args := client.Called()
	return args.Get(0).(map[string]int)
}

// k8sclient.NodeClient
func (client *MockClient) ClusterFailedNodeCount() int {
	args := client.Called()
	return args.Get(0).(int)
}

func (client *MockClient) ClusterNodeCount() int {
	args := client.Called()
	return args.Get(0).(int)
}

// k8sclient.EpClient
func (client *MockClient) ServiceToPodNum() map[k8sclient.Service]int {
	args := client.Called()
	return args.Get(0).(map[k8sclient.Service]int)
}

func (client *MockClient) Init() {
}

func (client *MockClient) Shutdown() {
}

func TestK8sAPIServer_Gather(t *testing.T) {
	hostName, err := os.Hostname()
	assert.NoError(t, err)
	plugin := &K8sAPIServer{
		NodeName: hostName,
		leading:  true,
	}

	k8sclient.Get = mockGet

	mockClient.On("NamespaceToRunningPodNum").Return(map[string]int{"default": 2})
	mockClient.On("ClusterFailedNodeCount").Return(1)
	mockClient.On("ClusterNodeCount").Return(1)
	mockClient.On("ServiceToPodNum").Return(map[k8sclient.Service]int{k8sclient.NewService("service1", "kube-system"): 1, k8sclient.NewService("service2", "kube-system"): 1})

	var acc testutil.Accumulator

	err = plugin.Gather(&acc)
	assert.NoError(t, err)

	/*
		tags: map[Timestamp:1557291396709 Type:Cluster], fields: map[cluster_failed_node_count:1 cluster_node_count:1],
		tags: map[Service:service2 Timestamp:1557291396709 Type:ClusterService], fields: map[service_number_of_running_pods:1],
		tags: map[Service:service1 Timestamp:1557291396709 Type:ClusterService], fields: map[service_number_of_running_pods:1],
		tags: map[Namespace:default Timestamp:1557291396709 Type:ClusterNamespace], fields: map[namespace_number_of_running_pods:2],
	*/
	for _, metric := range acc.Metrics {
		log.Printf("measurement: %v, tags: %v, fields: %v, time: %v\n", metric.Measurement, metric.Tags, metric.Fields, metric.Time)
		if metricType := metric.Tags[containerinsightscommon.MetricType]; metricType == containerinsightscommon.TypeCluster {
			assert.Equal(t, map[string]interface{}{"cluster_failed_node_count": 1, "cluster_node_count": 1}, metric.Fields)
		} else if metricType == containerinsightscommon.TypeClusterService {
			assert.Equal(t, map[string]interface{}{"service_number_of_running_pods": 1}, metric.Fields)
			if serviceTag := metric.Tags[containerinsightscommon.TypeService]; serviceTag != "service1" && serviceTag != "service2" {
				assert.Fail(t, "Expect to see a tag named as Service")
			}
			if namespaceTag := metric.Tags[containerinsightscommon.K8sNamespace]; namespaceTag != "kube-system" {
				assert.Fail(t, "Expect to see a tag named as Namespace")
			}
		} else if metricType == containerinsightscommon.TypeClusterNamespace {
			assert.Equal(t, map[string]interface{}{"namespace_number_of_running_pods": 2}, metric.Fields)
			assert.Equal(t, "default", metric.Tags[containerinsightscommon.K8sNamespace])
		} else {
			assert.Fail(t, "Unexpected metric type: "+metricType)
		}
	}

}
