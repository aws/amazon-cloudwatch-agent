// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package stores

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/influxdata/telegraf/metric"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"

	. "github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"github.com/aws/amazon-cloudwatch-agent/internal/k8sCommon/k8sclient"
	"github.com/aws/amazon-cloudwatch-agent/internal/mapWithExpiry"
)

func getBaseTestPodInfo() *corev1.Pod {
	podJson := `
{
  "kind": "PodList",
  "apiVersion": "v1",
  "metadata": {

  },
  "items": [
    {
      "metadata": {
        "name": "cpu-limit",
        "namespace": "default",
        "ownerReferences": [
            {
                "apiVersion": "apps/v1",
                "blockOwnerDeletion": true,
                "controller": true,
                "kind": "DaemonSet",
                "name": "DaemonSetTest",
                "uid": "36779a62-4aca-11e9-977b-0672b6c6fc94"
            }
        ],
        "selfLink": "/api/v1/namespaces/default/pods/cpu-limit",
        "uid": "764d01e1-2a2f-11e9-95ea-0a695d7ce286",
        "resourceVersion": "5671573",
        "creationTimestamp": "2019-02-06T16:51:34Z",
        "labels": {
          "app": "hello_test"
        },
        "annotations": {
          "kubernetes.io/config.seen": "2019-02-19T00:06:56.109155665Z",
          "kubernetes.io/config.source": "api"
        }
      },
      "spec": {
        "volumes": [
          {
            "name": "default-token-tlgw7",
            "secret": {
              "secretName": "default-token-tlgw7",
              "defaultMode": 420
            }
          }
        ],
        "containers": [
          {
            "name": "ubuntu",
            "image": "ubuntu",
            "command": [
              "/bin/bash"
            ],
            "args": [
              "-c",
              "sleep 300000000"
            ],
            "resources": {
              "limits": {
                "cpu": "10m",
                "memory": "50Mi"
              },
              "requests": {
                "cpu": "10m",
                "memory": "50Mi"
              }
            },
            "volumeMounts": [
              {
                "name": "default-token-tlgw7",
                "readOnly": true,
                "mountPath": "/var/run/secrets/kubernetes.io/serviceaccount"
              }
            ],
            "terminationMessagePath": "/dev/termination-log",
            "terminationMessagePolicy": "File",
            "imagePullPolicy": "Always"
          }
        ],
        "restartPolicy": "Always",
        "terminationGracePeriodSeconds": 30,
        "dnsPolicy": "ClusterFirst",
        "serviceAccountName": "default",
        "serviceAccount": "default",
        "nodeName": "ip-192-168-67-127.us-west-2.compute.internal",
        "securityContext": {

        },
        "schedulerName": "default-scheduler",
        "tolerations": [
          {
            "key": "node.kubernetes.io/not-ready",
            "operator": "Exists",
            "effect": "NoExecute",
            "tolerationSeconds": 300
          },
          {
            "key": "node.kubernetes.io/unreachable",
            "operator": "Exists",
            "effect": "NoExecute",
            "tolerationSeconds": 300
          }
        ],
        "priority": 0
      },
      "status": {
        "phase": "Running",
        "conditions": [
          {
            "type": "Initialized",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": "2019-02-06T16:51:34Z"
          },
          {
            "type": "Ready",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": "2019-02-06T16:51:43Z"
          },
          {
            "type": "ContainersReady",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": null
          },
          {
            "type": "PodScheduled",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": "2019-02-06T16:51:34Z"
          }
        ],
        "hostIP": "192.168.67.127",
        "podIP": "192.168.76.93",
        "startTime": "2019-02-06T16:51:34Z",
        "containerStatuses": [
          {
            "name": "ubuntu",
            "state": {
              "running": {
                "startedAt": "2019-02-06T16:51:42Z"
              }
            },
            "lastState": {

            },
            "ready": true,
            "restartCount": 0,
            "image": "ubuntu:latest",
            "imageID": "docker-pullable://ubuntu@sha256:7a47ccc3bbe8a451b500d2b53104868b46d60ee8f5b35a24b41a86077c650210",
            "containerID": "docker://637631e2634ea92c0c1aa5d24734cfe794f09c57933026592c12acafbaf6972c"
          }
        ],
        "qosClass": "Guaranteed"
      }
    }
  ]
}`
	pods := corev1.PodList{}
	err := json.Unmarshal([]byte(podJson), &pods)
	if err != nil {
		panic(fmt.Sprintf("unmarshal pod err %v", err))
	}

	return &pods.Items[0]
}

func TestPodStore_decorateCpu(t *testing.T) {
	podStore := &PodStore{nodeInfo: &nodeInfo{NodeCapacity: &NodeCapacity{MemCapacity: 400 * 1024 * 1024, CPUCapacity: 4}}}
	pod := getBaseTestPodInfo()

	tags := map[string]string{MetricType: TypePod}
	fields := map[string]interface{}{MetricName(TypePod, CpuTotal): float64(1)}

	m := metric.New("test", tags, fields, time.Now())

	podStore.decorateCpu(m, tags, pod)

	resultFields := m.Fields()
	assert.Equal(t, int64(10), resultFields["pod_cpu_request"])
	assert.Equal(t, int64(10), resultFields["pod_cpu_limit"])
	assert.Equal(t, float64(0.25), resultFields["pod_cpu_reserved_capacity"])
	assert.Equal(t, float64(10), resultFields["pod_cpu_utilization_over_pod_limit"])
	assert.Equal(t, float64(1), resultFields["pod_cpu_usage_total"])
	assert.Equal(t, float64(0.025), resultFields["pod_cpu_utilization"])
}

func TestPodStore_decorateMem(t *testing.T) {
	podStore := &PodStore{nodeInfo: &nodeInfo{NodeCapacity: &NodeCapacity{MemCapacity: 400 * 1024 * 1024, CPUCapacity: 4}}}
	pod := getBaseTestPodInfo()

	tags := map[string]string{MetricType: TypePod}
	fields := map[string]interface{}{MetricName(TypePod, MemWorkingset): int64(10 * 1024 * 1024)}

	m := metric.New("test", tags, fields, time.Now())

	podStore.decorateMem(m, tags, pod)

	resultFields := m.Fields()
	assert.Equal(t, int64(52428800), resultFields["pod_memory_request"])
	assert.Equal(t, int64(52428800), resultFields["pod_memory_limit"])
	assert.Equal(t, float64(12.5), resultFields["pod_memory_reserved_capacity"])
	assert.Equal(t, float64(20), resultFields["pod_memory_utilization_over_pod_limit"])
	assert.Equal(t, int64(10*1024*1024), resultFields["pod_memory_working_set"])
	assert.Equal(t, float64(2.5), resultFields["pod_memory_utilization"])
}

func TestPodStore_Decorate(t *testing.T) {
	podStore := &PodStore{nodeInfo: &nodeInfo{NodeCapacity: &NodeCapacity{MemCapacity: 400 * 1024 * 1024, CPUCapacity: 4}}, cache: mapWithExpiry.NewMapWithExpiry(PodsExpiry)}
	pod := getBaseTestPodInfo()
	pod.ObjectMeta.Annotations[ignoreAnnotation] = "true"
	podStore.setCachedEntry("namespace:test,podName:test", &cachedEntry{
		pod:      *pod,
		creation: time.Now(),
	})

	tags := map[string]string{MetricType: TypePod, K8sPodNameKey: "test", K8sNamespace: "test"}
	fields := map[string]interface{}{MetricName(TypePod, MemWorkingset): int64(10 * 1024 * 1024)}

	m := metric.New("test", tags, fields, time.Now())
	kubernetesBlob := map[string]interface{}{}

	assert.False(t, podStore.Decorate(m, kubernetesBlob))
}

func TestPodStore_addContainerCount(t *testing.T) {
	pod := getBaseTestPodInfo()
	tags := map[string]string{MetricType: TypePod}
	m := metric.New("test", tags, map[string]interface{}{}, time.Now())
	addContainerCount(m, tags, pod)
	assert.Equal(t, int64(1), m.Fields()[MetricName(TypePod, RunningContainerCount)])
	assert.Equal(t, int64(1), m.Fields()[MetricName(TypePod, ContainerCount)])

	pod.Status.ContainerStatuses[0].State.Running = nil
	addContainerCount(m, tags, pod)
	assert.Equal(t, int64(0), m.Fields()[MetricName(TypePod, RunningContainerCount)])
	assert.Equal(t, int64(1), m.Fields()[MetricName(TypePod, ContainerCount)])
}

func TestPodStore_addStatus(t *testing.T) {
	pod := getBaseTestPodInfo()
	tags := map[string]string{MetricType: TypePod, K8sNamespace: "default", K8sPodNameKey: "cpu-limit"}
	m := metric.New("test", tags, map[string]interface{}{}, time.Now())
	podStore := &PodStore{prevMeasurements: make(map[string]*mapWithExpiry.MapWithExpiry)}

	podStore.addStatus(m, tags, pod)
	assert.Equal(t, "Running", m.Fields()[PodStatus].(string))
	_, ok := m.Fields()[MetricName(TypePod, ContainerRestartCount)]
	assert.False(t, ok)

	tags = map[string]string{MetricType: TypeContainer, K8sNamespace: "default", K8sPodNameKey: "cpu-limit", ContainerNamekey: "ubuntu"}
	m = metric.New("test", tags, map[string]interface{}{}, time.Now())
	podStore.addStatus(m, tags, pod)
	assert.Equal(t, "Running", m.Fields()[ContainerStatus].(string))
	_, ok = m.Fields()[ContainerRestartCount]
	assert.False(t, ok)

	pod.Status.ContainerStatuses[0].State.Running = nil
	pod.Status.ContainerStatuses[0].State.Terminated = &corev1.ContainerStateTerminated{}
	pod.Status.ContainerStatuses[0].LastTerminationState.Terminated = &corev1.ContainerStateTerminated{Reason: "OOMKilled"}
	pod.Status.ContainerStatuses[0].RestartCount = 1
	pod.Status.Phase = "Succeeded"

	tags = map[string]string{MetricType: TypePod, K8sNamespace: "default", K8sPodNameKey: "cpu-limit"}
	m = metric.New("test", tags, map[string]interface{}{}, time.Now())
	podStore.addStatus(m, tags, pod)
	assert.Equal(t, "Succeeded", m.Fields()[PodStatus].(string))
	assert.Equal(t, int64(1), m.Fields()[MetricName(TypePod, ContainerRestartCount)].(int64))

	tags = map[string]string{MetricType: TypeContainer, K8sNamespace: "default", K8sPodNameKey: "cpu-limit", ContainerNamekey: "ubuntu"}
	m = metric.New("test", tags, map[string]interface{}{}, time.Now())
	podStore.addStatus(m, tags, pod)
	assert.Equal(t, "Terminated", m.Fields()[ContainerStatus].(string))
	assert.Equal(t, "OOMKilled", m.Fields()[ContainerLastTerminationReason].(string))
	assert.Equal(t, int64(1), m.Fields()[ContainerRestartCount].(int64))

	// test delta of restartCount
	pod.Status.ContainerStatuses[0].RestartCount = 3
	tags = map[string]string{MetricType: TypePod, K8sNamespace: "default", K8sPodNameKey: "cpu-limit"}
	m = metric.New("test", tags, map[string]interface{}{}, time.Now())
	podStore.addStatus(m, tags, pod)
	assert.Equal(t, int64(2), m.Fields()[MetricName(TypePod, ContainerRestartCount)].(int64))

	tags = map[string]string{MetricType: TypeContainer, K8sNamespace: "default", K8sPodNameKey: "cpu-limit", ContainerNamekey: "ubuntu"}
	m = metric.New("test", tags, map[string]interface{}{}, time.Now())
	podStore.addStatus(m, tags, pod)
	assert.Equal(t, int64(2), m.Fields()[ContainerRestartCount].(int64))
}

func TestPodStore_addContainerId(t *testing.T) {
	pod := getBaseTestPodInfo()
	tags := map[string]string{ContainerNamekey: "ubuntu", ContainerIdkey: "123"}
	m := metric.New("test", tags, map[string]interface{}{}, time.Now())
	kubernetesBlob := map[string]interface{}{}
	addContainerId(pod, tags, m, kubernetesBlob)

	expected := map[string]interface{}{}
	expected["docker"] = map[string]string{"container_id": "637631e2634ea92c0c1aa5d24734cfe794f09c57933026592c12acafbaf6972c"}
	assert.Equal(t, expected, kubernetesBlob)
	assert.Equal(t, map[string]string{ContainerNamekey: "ubuntu"}, m.Tags())

	tags = map[string]string{ContainerNamekey: "notUbuntu", ContainerIdkey: "123"}
	m = metric.New("test", tags, map[string]interface{}{}, time.Now())
	kubernetesBlob = map[string]interface{}{}
	addContainerId(pod, tags, m, kubernetesBlob)

	expected = map[string]interface{}{}
	expected["container_id"] = "123"
	assert.Equal(t, expected, kubernetesBlob)
	assert.Equal(t, map[string]string{ContainerNamekey: "notUbuntu"}, m.Tags())
}

func TestPodStore_addLabel(t *testing.T) {
	pod := getBaseTestPodInfo()
	kubernetesBlob := map[string]interface{}{}
	addLabels(pod, kubernetesBlob)
	expected := map[string]interface{}{}
	expected["labels"] = map[string]string{"app": "hello_test"}
	assert.Equal(t, expected, kubernetesBlob)
}

// Mock client start
var mockClient = new(MockClient)

var mockK8sClient = &k8sclient.K8sClient{
	ReplicaSet: mockClient,
}

func mockGet() *k8sclient.K8sClient {
	return mockK8sClient
}

type MockClient struct {
	k8sclient.ReplicaSetClient

	mock.Mock
}

// k8sclient.ReplicaSetClient
func (client *MockClient) ReplicaSetToDeployment() map[string]string {
	args := client.Called()
	return args.Get(0).(map[string]string)
}

func (client *MockClient) Init() {
}

func (client *MockClient) Shutdown() {
}

//
// Mock client end
//

// Mock client 2 start
var mockClient2 = new(MockClient2)

var mockK8sClient2 = &k8sclient.K8sClient{
	ReplicaSet: mockClient2,
}

func mockGet2() *k8sclient.K8sClient {
	return mockK8sClient2
}

type MockClient2 struct {
	k8sclient.ReplicaSetClient

	mock.Mock
}

// k8sclient.ReplicaSetClient
func (client *MockClient2) ReplicaSetToDeployment() map[string]string {
	args := client.Called()
	return args.Get(0).(map[string]string)
}

func (client *MockClient2) Init() {
}

func (client *MockClient2) Shutdown() {
}

//
// Mock client 2 end
//

func TestGetJobNamePrefix(t *testing.T) {
	assert.Equal(t, "abcd", getJobNamePrefix("abcd-efg"))
	assert.Equal(t, "abcd", getJobNamePrefix("abcd.efg"))
	assert.Equal(t, "abcd", getJobNamePrefix("abcd-e.fg"))
	assert.Equal(t, "abc", getJobNamePrefix("abc.d-efg"))
	assert.Equal(t, "abcd", getJobNamePrefix("abcd-.efg"))
	assert.Equal(t, "abcd", getJobNamePrefix("abcd.-efg"))
	assert.Equal(t, "abcdefg", getJobNamePrefix("abcdefg"))
	assert.Equal(t, "abcdefg", getJobNamePrefix("abcdefg-"))
	assert.Equal(t, "", getJobNamePrefix(".abcd-efg"))
	assert.Equal(t, "", getJobNamePrefix(""))
}

func TestPodStore_addPodOwnersAndPodNameFallback(t *testing.T) {
	k8sclient.Get = mockGet2
	mockClient2.On("ReplicaSetToDeployment").Return(map[string]string{})

	podStore := &PodStore{}
	pod := getBaseTestPodInfo()
	tags := map[string]string{MetricType: TypePod, ContainerNamekey: "ubuntu"}

	// Test ReplicaSet
	m := metric.New("test", tags, map[string]interface{}{}, time.Now())
	rsName := "ReplicaSetTest"
	suffix := "-42kcz"
	pod.OwnerReferences[0].Kind = ReplicaSet
	pod.OwnerReferences[0].Name = rsName + suffix
	kubernetesBlob := map[string]interface{}{}
	podStore.addPodOwnersAndPodName(m, pod, kubernetesBlob)
	expectedOwner := map[string]interface{}{}
	expectedOwner["pod_owners"] = []Owner{{OwnerKind: Deployment, OwnerName: rsName}}
	expectedOwnerName := rsName
	assert.Equal(t, expectedOwnerName, m.Tags()[PodNameKey])
	assert.Equal(t, expectedOwner, kubernetesBlob)

	// Test Job
	m = metric.New("test", tags, map[string]interface{}{}, time.Now())
	jobName := "Job"
	suffix = "-0123456789"
	pod.OwnerReferences[0].Kind = Job
	pod.OwnerReferences[0].Name = jobName + suffix
	kubernetesBlob = map[string]interface{}{}
	podStore.addPodOwnersAndPodName(m, pod, kubernetesBlob)
	expectedOwner["pod_owners"] = []Owner{{OwnerKind: CronJob, OwnerName: jobName}}
	expectedOwnerName = jobName
	assert.Equal(t, expectedOwnerName, m.Tags()[PodNameKey])
	assert.Equal(t, expectedOwner, kubernetesBlob)
}

func TestPodStore_addPodOwnersAndPodName(t *testing.T) {
	k8sclient.Get = mockGet
	mockClient.On("ReplicaSetToDeployment").Return(map[string]string{"DeploymentTest-sftrz2785": "DeploymentTest"})

	podStore := &PodStore{}

	pod := getBaseTestPodInfo()
	tags := map[string]string{MetricType: TypePod, ContainerNamekey: "ubuntu"}
	m := metric.New("test", tags, map[string]interface{}{}, time.Now())

	// Test DaemonSet
	kubernetesBlob := map[string]interface{}{}
	podStore.addPodOwnersAndPodName(m, pod, kubernetesBlob)

	expectedOwner := map[string]interface{}{}
	expectedOwner["pod_owners"] = []Owner{{OwnerKind: DaemonSet, OwnerName: "DaemonSetTest"}}
	expectedOwnerName := "DaemonSetTest"
	assert.Equal(t, expectedOwnerName, m.Tags()[PodNameKey])
	assert.Equal(t, expectedOwner, kubernetesBlob)

	// Test ReplicaSet
	m = metric.New("test", tags, map[string]interface{}{}, time.Now())
	rsName := "ReplicaSetTest"
	pod.OwnerReferences[0].Kind = ReplicaSet
	pod.OwnerReferences[0].Name = rsName
	kubernetesBlob = map[string]interface{}{}
	podStore.addPodOwnersAndPodName(m, pod, kubernetesBlob)
	expectedOwner["pod_owners"] = []Owner{{OwnerKind: ReplicaSet, OwnerName: rsName}}
	expectedOwnerName = rsName
	assert.Equal(t, expectedOwnerName, m.Tags()[PodNameKey])
	assert.Equal(t, expectedOwner, kubernetesBlob)

	// Test StatefulSet
	m = metric.New("test", tags, map[string]interface{}{}, time.Now())
	ssName := "StatefulSetTest"
	pod.OwnerReferences[0].Kind = StatefulSet
	pod.OwnerReferences[0].Name = ssName
	kubernetesBlob = map[string]interface{}{}
	podStore.addPodOwnersAndPodName(m, pod, kubernetesBlob)
	expectedOwner["pod_owners"] = []Owner{{OwnerKind: StatefulSet, OwnerName: ssName}}
	expectedOwnerName = "cpu-limit"
	assert.Equal(t, expectedOwnerName, m.Tags()[PodNameKey])
	assert.Equal(t, expectedOwner, kubernetesBlob)

	// Test ReplicationController
	rcName := "ReplicationControllerTest"
	pod.OwnerReferences[0].Kind = ReplicationController
	pod.OwnerReferences[0].Name = rcName
	kubernetesBlob = map[string]interface{}{}
	podStore.addPodOwnersAndPodName(m, pod, kubernetesBlob)
	expectedOwner["pod_owners"] = []Owner{{OwnerKind: ReplicationController, OwnerName: rcName}}
	expectedOwnerName = rcName
	assert.Equal(t, expectedOwnerName, m.Tags()[PodNameKey])
	assert.Equal(t, expectedOwner, kubernetesBlob)

	// Test Job
	podStore.prefFullPodName = true
	m = metric.New("test", tags, map[string]interface{}{}, time.Now())
	jobName := "JobTest"
	pod.OwnerReferences[0].Kind = Job
	surfixHash := ".088123x12"
	pod.OwnerReferences[0].Name = jobName + surfixHash
	kubernetesBlob = map[string]interface{}{}
	podStore.addPodOwnersAndPodName(m, pod, kubernetesBlob)
	expectedOwner["pod_owners"] = []Owner{{OwnerKind: Job, OwnerName: jobName + surfixHash}}
	expectedOwnerName = jobName + surfixHash
	assert.Equal(t, expectedOwnerName, m.Tags()[PodNameKey])
	assert.Equal(t, expectedOwner, kubernetesBlob)

	podStore.prefFullPodName = false
	kubernetesBlob = map[string]interface{}{}
	podStore.addPodOwnersAndPodName(m, pod, kubernetesBlob)
	expectedOwner["pod_owners"] = []Owner{{OwnerKind: Job, OwnerName: jobName}}
	expectedOwnerName = jobName
	assert.Equal(t, expectedOwnerName, m.Tags()[PodNameKey])
	assert.Equal(t, expectedOwner, kubernetesBlob)

	// Test Deployment
	m = metric.New("test", tags, map[string]interface{}{}, time.Now())
	dpName := "DeploymentTest"
	pod.OwnerReferences[0].Kind = ReplicaSet
	pod.OwnerReferences[0].Name = dpName + "-sftrz2785"
	kubernetesBlob = map[string]interface{}{}
	podStore.addPodOwnersAndPodName(m, pod, kubernetesBlob)
	expectedOwner["pod_owners"] = []Owner{{OwnerKind: Deployment, OwnerName: dpName}}
	expectedOwnerName = dpName
	assert.Equal(t, expectedOwnerName, m.Tags()[PodNameKey])
	assert.Equal(t, expectedOwner, kubernetesBlob)

	// Test CronJob
	m = metric.New("test", tags, map[string]interface{}{}, time.Now())
	cjName := "CronJobTest"
	pod.OwnerReferences[0].Kind = Job
	pod.OwnerReferences[0].Name = cjName + "-1556582405"
	kubernetesBlob = map[string]interface{}{}
	podStore.addPodOwnersAndPodName(m, pod, kubernetesBlob)
	expectedOwner["pod_owners"] = []Owner{{OwnerKind: CronJob, OwnerName: cjName}}
	expectedOwnerName = cjName
	assert.Equal(t, expectedOwnerName, m.Tags()[PodNameKey])
	assert.Equal(t, expectedOwner, kubernetesBlob)

	// Test kube-proxy created in kops
	podStore.prefFullPodName = true
	m = metric.New("test", tags, map[string]interface{}{}, time.Now())
	kpName := kubeProxy + "-xyz1"
	pod.OwnerReferences = nil
	pod.Name = kpName
	kubernetesBlob = map[string]interface{}{}
	podStore.addPodOwnersAndPodName(m, pod, kubernetesBlob)
	assert.Equal(t, kpName, m.Tags()[PodNameKey])
	assert.True(t, len(kubernetesBlob) == 0)

	podStore.prefFullPodName = false
	m = metric.New("test", tags, map[string]interface{}{}, time.Now())
	pod.OwnerReferences = nil
	pod.Name = kpName
	kubernetesBlob = map[string]interface{}{}
	podStore.addPodOwnersAndPodName(m, pod, kubernetesBlob)
	assert.Equal(t, kubeProxy, m.Tags()[PodNameKey])
	assert.True(t, len(kubernetesBlob) == 0)
}

func TestPodStore_refreshInternal(t *testing.T) {
	pod := getBaseTestPodInfo()
	podList := []corev1.Pod{*pod}

	podStore := &PodStore{cache: mapWithExpiry.NewMapWithExpiry(time.Minute), nodeInfo: &nodeInfo{NodeCapacity: &NodeCapacity{MemCapacity: 400 * 1024 * 1024, CPUCapacity: 4}}}
	podStore.refreshInternal(time.Now(), podList)

	assert.Equal(t, int64(10), podStore.nodeInfo.nodeStats.cpuReq)
	assert.Equal(t, int64(50*1024*1024), podStore.nodeInfo.nodeStats.memReq)
	assert.Equal(t, 1, podStore.nodeInfo.nodeStats.podCnt)
	assert.Equal(t, 1, podStore.nodeInfo.nodeStats.containerCnt)
	assert.Equal(t, 1, podStore.cache.Size())
}

func TestPodStore_decorateNode(t *testing.T) {
	pod := getBaseTestPodInfo()
	podList := []corev1.Pod{*pod}

	podStore := &PodStore{cache: mapWithExpiry.NewMapWithExpiry(time.Minute), nodeInfo: &nodeInfo{NodeCapacity: &NodeCapacity{MemCapacity: 400 * 1024 * 1024, CPUCapacity: 4}}}
	podStore.refreshInternal(time.Now(), podList)

	tags := map[string]string{MetricType: TypeNode}
	fields := map[string]interface{}{MetricName(TypeNode, CpuTotal): float64(100), MetricName(TypeNode, MemWorkingset): uint64(100 * 1024 * 1024)}

	m := metric.New("test", tags, fields, time.Now())
	podStore.decorateNode(m)

	resultFields := m.Fields()
	assert.Equal(t, int64(10), resultFields["node_cpu_request"])
	assert.Equal(t, int64(4000), resultFields["node_cpu_limit"])
	assert.Equal(t, float64(0.25), resultFields["node_cpu_reserved_capacity"])
	assert.Equal(t, float64(100), resultFields["node_cpu_usage_total"])
	assert.Equal(t, float64(2.5), resultFields["node_cpu_utilization"])

	assert.Equal(t, int64(50*1024*1024), resultFields["node_memory_request"])
	assert.Equal(t, int64(400*1024*1024), resultFields["node_memory_limit"])
	assert.Equal(t, float64(12.5), resultFields["node_memory_reserved_capacity"])
	assert.Equal(t, uint64(100*1024*1024), resultFields["node_memory_working_set"])
	assert.Equal(t, float64(25), resultFields["node_memory_utilization"])

	assert.Equal(t, int64(1), resultFields["node_number_of_running_containers"])
	assert.Equal(t, int64(1), resultFields["node_number_of_running_pods"])
}

func TestPodStore_decorateDiskDevice(t *testing.T) {
	nodeInfo := &nodeInfo{NodeCapacity: &NodeCapacity{MemCapacity: 400 * 1024 * 1024, CPUCapacity: 4}, ebsIds: mapWithExpiry.NewMapWithExpiry(2 * refreshInterval)}
	podStore := &PodStore{nodeInfo: nodeInfo}
	podStore.nodeInfo.ebsIds.Set("/dev/xvda", "aws://us-west-2b/vol-0d9f0816149eb2050")

	tags := map[string]string{MetricType: TypeNodeFS, DiskDev: "/dev/xvda"}

	m := metric.New("test", tags, nil, time.Now())
	podStore.decorateDiskDevice(m, tags)

	assert.Equal(t, "aws://us-west-2b/vol-0d9f0816149eb2050", m.Tags()[EbsVolumeId])
}
