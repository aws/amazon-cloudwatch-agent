// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resolver

import (
	"sync"
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent/internal/k8sCommon/k8sclient"
)

func newPodWatcherForTesting(ipToPod, podToWorkloadAndNamespace, workloadAndNamespaceToLabels *sync.Map, workloadPodCount map[string]int) *podWatcher {
	logger, _ := zap.NewDevelopment()
	return &podWatcher{
		ipToPod:                      ipToPod,
		podToWorkloadAndNamespace:    podToWorkloadAndNamespace,
		workloadAndNamespaceToLabels: workloadAndNamespaceToLabels,
		workloadPodCount:             workloadPodCount,
		logger:                       logger,
		informer:                     nil,
		deleter:                      mockDeleter,
	}
}

func TestOnAddOrUpdatePod(t *testing.T) {
	t.Run("pod with both PodIP and HostIP", func(t *testing.T) {
		ipToPod := &sync.Map{}
		podToWorkloadAndNamespace := &sync.Map{}
		workloadAndNamespaceToLabels := &sync.Map{}
		workloadPodCount := map[string]int{}

		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPod",
				Namespace: "testNamespace",
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind: "ReplicaSet",
						Name: "testDeployment-598b89cd8d",
					},
				},
			},
			Status: corev1.PodStatus{
				PodIP:  "1.2.3.4",
				HostIP: "5.6.7.8",
			},
		}

		poWatcher := newPodWatcherForTesting(ipToPod, podToWorkloadAndNamespace, workloadAndNamespaceToLabels, workloadPodCount)
		poWatcher.onAddOrUpdatePod(pod, nil)

		// Test the mappings in ipToPod
		if podName, _ := ipToPod.Load("1.2.3.4"); podName != "testPod" {
			t.Errorf("ipToPod was incorrect, got: %s, want: %s.", podName, "testPod")
		}

		// Test the mapping in podToWorkloadAndNamespace
		if depAndNamespace, _ := podToWorkloadAndNamespace.Load("testPod"); depAndNamespace != "testDeployment@testNamespace" {
			t.Errorf("podToWorkloadAndNamespace was incorrect, got: %s, want: %s.", depAndNamespace, "testDeployment@testNamespace")
		}

		// Test the count in workloadPodCount
		if count := workloadPodCount["testDeployment@testNamespace"]; count != 1 {
			t.Errorf("workloadPodCount was incorrect, got: %d, want: %d.", count, 1)
		}
	})

	t.Run("pod with only HostIP", func(t *testing.T) {
		ipToPod := &sync.Map{}
		podToWorkloadAndNamespace := &sync.Map{}
		workloadAndNamespaceToLabels := &sync.Map{}
		workloadPodCount := map[string]int{}

		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPod",
				Namespace: "testNamespace",
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind: "ReplicaSet",
						Name: "testDeployment-7b74958fb8",
					},
				},
			},
			Status: corev1.PodStatus{
				HostIP: "5.6.7.8",
			},
			Spec: corev1.PodSpec{
				HostNetwork: true,
				Containers: []corev1.Container{
					{
						Ports: []corev1.ContainerPort{
							{
								HostPort: int32(8080),
							},
						},
					},
				},
			},
		}

		poWatcher := newPodWatcherForTesting(ipToPod, podToWorkloadAndNamespace, workloadAndNamespaceToLabels, workloadPodCount)
		poWatcher.onAddOrUpdatePod(pod, nil)

		// Test the mappings in ipToPod
		if podName, _ := ipToPod.Load("5.6.7.8:8080"); podName != "testPod" {
			t.Errorf("ipToPod was incorrect, got: %s, want: %s.", podName, "testPod")
		}

		// Test the mapping in podToWorkloadAndNamespace
		if depAndNamespace, _ := podToWorkloadAndNamespace.Load("testPod"); depAndNamespace != "testDeployment@testNamespace" {
			t.Errorf("podToWorkloadAndNamespace was incorrect, got: %s, want: %s.", depAndNamespace, "testDeployment@testNamespace")
		}

		// Test the count in workloadPodCount
		if count := workloadPodCount["testDeployment@testNamespace"]; count != 1 {
			t.Errorf("workloadPodCount was incorrect, got: %d, want: %d.", count, 1)
		}
	})

	t.Run("pod updated with different set of labels", func(t *testing.T) {
		ipToPod := &sync.Map{}
		podToWorkloadAndNamespace := &sync.Map{}
		workloadAndNamespaceToLabels := &sync.Map{}

		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPod",
				Namespace: "testNamespace",
				Labels: map[string]string{
					"label1": "value1",
					"label2": "value2",
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind: "ReplicaSet",
						Name: "testDeployment-5d68bc5f49",
					},
				},
			},
			Status: corev1.PodStatus{
				HostIP: "5.6.7.8",
			},
			Spec: corev1.PodSpec{
				HostNetwork: true,
				Containers: []corev1.Container{
					{
						Ports: []corev1.ContainerPort{
							{HostPort: 8080},
						},
					},
				},
			},
		}

		// add the pod
		poWatcher := newPodWatcherForTesting(ipToPod, podToWorkloadAndNamespace, workloadAndNamespaceToLabels, map[string]int{})
		poWatcher.onAddOrUpdatePod(pod, nil)

		// Test the mappings in ipToPod
		if podName, ok := ipToPod.Load("5.6.7.8:8080"); !ok && podName != "testPod" {
			t.Errorf("ipToPod[%s] was incorrect, got: %s, want: %s.", "5.6.7.8:8080", podName, "testPod")
		}

		// Test the mapping in workloadAndNamespaceToLabels
		labels, _ := workloadAndNamespaceToLabels.Load("testDeployment@testNamespace")
		expectedLabels := []string{"label1=value1", "label2=value2"}
		for _, label := range expectedLabels {
			if !labels.(mapset.Set[string]).Contains(label) {
				t.Errorf("deploymentAndNamespaceToLabels was incorrect, got: %v, want: %s.", labels, label)
			}
		}

		pod2 := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPod",
				Namespace: "testNamespace",
				Labels: map[string]string{
					"label1": "value1",
					"label2": "value2",
					"label3": "value3",
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind: "ReplicaSet",
						Name: "testDeployment-5d68bc5f49",
					},
				},
			},
			Status: corev1.PodStatus{
				PodIP:  "1.2.3.4",
				HostIP: "5.6.7.8",
			},
			Spec: corev1.PodSpec{
				HostNetwork: true,
				Containers: []corev1.Container{
					{
						Ports: []corev1.ContainerPort{
							{HostPort: 8080},
						},
					},
				},
			},
		}

		// add the pod
		poWatcher.onAddOrUpdatePod(pod2, pod)

		// Test the mappings in ipToPod
		if podName, ok := ipToPod.Load("5.6.7.8:8080"); !ok && podName != "testPod" {
			t.Errorf("ipToPod[%s] was incorrect, got: %s, want: %s.", "5.6.7.8:8080", podName, "testPod")
		}

		if podName, ok := ipToPod.Load("1.2.3.4"); !ok && podName != "testPod" {
			t.Errorf("ipToPod[%s] was incorrect, got: %s, want: %s.", "1.2.3.4", podName, "testPod")
		}
		// Test the mapping in workloadAndNamespaceToLabels
		labels, _ = workloadAndNamespaceToLabels.Load("testDeployment@testNamespace")
		expectedLabels = []string{"label1=value1", "label2=value2", "label3=value3"}
		for _, label := range expectedLabels {
			if !labels.(mapset.Set[string]).Contains(label) {
				t.Errorf("workloadAndNamespaceToLabels was incorrect, got: %v, want: %s.", labels, label)
			}
		}
	})
}

func TestOnDeletePod(t *testing.T) {
	t.Run("pod with both PodIP and HostIP", func(t *testing.T) {
		ipToPod := &sync.Map{}
		podToWorkloadAndNamespace := &sync.Map{}
		workloadAndNamespaceToLabels := &sync.Map{}
		workloadPodCount := map[string]int{}

		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPod",
				Namespace: "testNamespace",
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind: "ReplicaSet",
						Name: "testDeployment-xyz",
					},
				},
			},
			Status: corev1.PodStatus{
				PodIP:  "1.2.3.4",
				HostIP: "5.6.7.8",
			},
		}

		// Assume the pod has already been added
		ipToPod.Store(pod.Status.PodIP, pod.Name)
		ipToPod.Store(pod.Status.HostIP, pod.Name)
		podToWorkloadAndNamespace.Store(pod.Name, "testDeployment@testNamespace")
		workloadAndNamespaceToLabels.Store("testDeployment@testNamespace", "testLabels")
		workloadPodCount["testDeployment@testNamespace"] = 1

		poWatcher := newPodWatcherForTesting(ipToPod, podToWorkloadAndNamespace, workloadAndNamespaceToLabels, workloadPodCount)
		poWatcher.onDeletePod(pod)

		// Test if the entries in ipToPod and podToWorkloadAndNamespace have been deleted
		if _, ok := ipToPod.Load("1.2.3.4"); ok {
			t.Errorf("ipToPod deletion was incorrect, key: %s still exists", "1.2.3.4")
		}

		if _, ok := podToWorkloadAndNamespace.Load("testPod"); ok {
			t.Errorf("podToWorkloadAndNamespace deletion was incorrect, key: %s still exists", "testPod")
		}

		// Test if the count in workloadPodCount has been decremented and the entry in workloadAndNamespaceToLabels has been deleted
		if count := workloadPodCount["testDeployment@testNamespace"]; count != 0 {
			t.Errorf("workloadPodCount was incorrect, got: %d, want: %d.", count, 0)
		}

		if _, ok := workloadAndNamespaceToLabels.Load("testDeployment@testNamespace"); ok {
			t.Errorf("workloadAndNamespaceToLabels deletion was incorrect, key: %s still exists", "testDeployment@testNamespace")
		}
	})

	t.Run("pod with only HostIP and some network ports", func(t *testing.T) {
		ipToPod := &sync.Map{}
		podToWorkloadAndNamespace := &sync.Map{}
		workloadAndNamespaceToLabels := &sync.Map{}
		workloadPodCount := map[string]int{}

		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPod",
				Namespace: "testNamespace",
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind: "ReplicaSet",
						Name: "testDeployment-xyz",
					},
				},
			},
			Status: corev1.PodStatus{
				HostIP: "5.6.7.8",
			},
			Spec: corev1.PodSpec{
				HostNetwork: true,
				Containers: []corev1.Container{
					{
						Ports: []corev1.ContainerPort{
							{
								HostPort: int32(8080),
							},
						},
					},
				},
			},
		}

		// Assume the pod has already been added
		ipToPod.Store(pod.Status.HostIP, pod.Name)
		ipToPod.Store(pod.Status.HostIP+":8080", pod.Name)
		podToWorkloadAndNamespace.Store(pod.Name, "testDeployment@testNamespace")
		workloadAndNamespaceToLabels.Store("testDeployment@testNamespace", "testLabels")
		workloadPodCount["testDeployment@testNamespace"] = 1

		poWatcher := newPodWatcherForTesting(ipToPod, podToWorkloadAndNamespace, workloadAndNamespaceToLabels, workloadPodCount)
		poWatcher.onDeletePod(pod)

		// Test if the entries in ipToPod and podToWorkloadAndNamespace have been deleted
		if _, ok := ipToPod.Load("5.6.7.8:8080"); ok {
			t.Errorf("ipToPod deletion was incorrect, key: %s still exists", "5.6.7.8:8080")
		}

		if _, ok := podToWorkloadAndNamespace.Load("testPod"); ok {
			t.Errorf("podToDeploymentAndNamespace deletion was incorrect, key: %s still exists", "testPod")
		}

		// Test if the count in workloadPodCount has been decremented and the entry in workloadAndNamespaceToLabels has been deleted
		if count := workloadPodCount["testDeployment@testNamespace"]; count != 0 {
			t.Errorf("workloadPodCount was incorrect, got: %d, want: %d.", count, 0)
		}

		if _, ok := workloadAndNamespaceToLabels.Load("testDeployment@testNamespace"); ok {
			t.Errorf("workloadAndNamespaceToLabels deletion was incorrect, key: %s still exists", "testDeployment@testNamespace")
		}
	})
}

func TestHandlePodUpdate(t *testing.T) {
	testCases := []struct {
		name            string
		oldPod          *corev1.Pod
		newPod          *corev1.Pod
		initialIPToPod  map[string]string
		expectedIPToPod map[string]string
	}{
		{
			name: "Old and New Pod Do Not Use Host Network, Different Pod IPs",
			oldPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mypod",
				},
				Status: corev1.PodStatus{
					PodIP: "10.0.0.3",
				},
				Spec: corev1.PodSpec{
					HostNetwork: false,
				},
			},
			newPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mypod",
				},
				Status: corev1.PodStatus{
					PodIP: "10.0.0.4",
				},
				Spec: corev1.PodSpec{
					HostNetwork: false,
				},
			},
			initialIPToPod: map[string]string{
				"10.0.0.3": "mypod",
			},
			expectedIPToPod: map[string]string{
				"10.0.0.4": "mypod",
			},
		},
		{
			name: "Old Pod Has Empty PodIP, New Pod Does Not Use Host Network, Non-Empty Pod IP",
			oldPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mypod",
				},
				Status: corev1.PodStatus{
					PodIP: "",
				},
				Spec: corev1.PodSpec{
					HostNetwork: false,
				},
			},
			newPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mypod",
				},
				Status: corev1.PodStatus{
					PodIP: "10.0.0.5",
				},
				Spec: corev1.PodSpec{
					HostNetwork: false,
				},
			},
			initialIPToPod: map[string]string{},
			expectedIPToPod: map[string]string{
				"10.0.0.5": "mypod",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ipToPod := &sync.Map{}
			// Initialize ipToPod map
			for k, v := range tc.initialIPToPod {
				ipToPod.Store(k, v)
			}
			poWatcher := newPodWatcherForTesting(ipToPod, nil, nil, map[string]int{})
			poWatcher.handlePodUpdate(tc.newPod, tc.oldPod)

			// Now validate that ipToPod map has been updated correctly
			for key, expectedValue := range tc.expectedIPToPod {
				val, ok := ipToPod.Load(key)
				if !ok || val.(string) != expectedValue {
					t.Errorf("Expected record for %v to be %v, got %v", key, expectedValue, val)
				}
			}
			// Validate that old keys have been removed
			for key := range tc.initialIPToPod {
				if _, ok := tc.expectedIPToPod[key]; !ok {
					if _, ok := ipToPod.Load(key); ok {
						t.Errorf("Expected record for %v to be removed, but it was not", key)
					}
				}
			}
		})
	}
}

func TestFilterPodIPFields(t *testing.T) {
	meta := metav1.ObjectMeta{
		Name:      "test",
		Namespace: "default",
		Labels: map[string]string{
			"name": "app",
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: meta,
		Spec: corev1.PodSpec{
			HostNetwork: true,
			Containers: []corev1.Container{
				{},
			},
		},
		Status: corev1.PodStatus{},
	}
	newPod, err := minimizePod(pod)
	assert.Nil(t, err)
	assert.Empty(t, k8sclient.GetHostNetworkPorts(newPod.(*corev1.Pod)))

	podStatus := corev1.PodStatus{
		PodIP: "192.168.0.12",
		HostIPs: []corev1.HostIP{
			{
				IP: "132.168.3.12",
			},
		},
	}
	pod = &corev1.Pod{
		ObjectMeta: meta,
		Spec: corev1.PodSpec{
			HostNetwork: true,
			Containers: []corev1.Container{
				{
					Ports: []corev1.ContainerPort{
						{HostPort: 8080},
					},
				},
			},
		},
		Status: podStatus,
	}
	newPod, err = minimizePod(pod)
	assert.Nil(t, err)
	assert.Equal(t, "app", newPod.(*corev1.Pod).Labels["name"])
	assert.Equal(t, []string{"8080"}, k8sclient.GetHostNetworkPorts(newPod.(*corev1.Pod)))
	assert.Equal(t, podStatus, newPod.(*corev1.Pod).Status)

	pod = &corev1.Pod{
		Spec: corev1.PodSpec{
			HostNetwork: true,
			Containers: []corev1.Container{
				{
					Ports: []corev1.ContainerPort{
						{HostPort: 8080},
						{HostPort: 8081},
					},
				},
			},
		},
		Status: podStatus,
	}
	newPod, err = minimizePod(pod)
	assert.Nil(t, err)
	assert.Equal(t, []string{"8080", "8081"}, k8sclient.GetHostNetworkPorts(newPod.(*corev1.Pod)))
	assert.Equal(t, podStatus, newPod.(*corev1.Pod).Status)
}
