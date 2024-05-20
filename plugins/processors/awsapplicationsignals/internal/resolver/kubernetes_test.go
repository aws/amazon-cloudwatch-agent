// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resolver

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/collector/semconv/v1.22.0"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/common"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/config"
	attr "github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/internal/attributes"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/eksdetector"
)

// MockDeleter deletes a key immediately, useful for testing.
type MockDeleter struct{}

func (md *MockDeleter) DeleteWithDelay(m *sync.Map, key interface{}) {
	m.Delete(key)
}

var mockDeleter = &MockDeleter{}

// TestAttachNamespace function
func TestAttachNamespace(t *testing.T) {
	result := attachNamespace("testResource", "testNamespace")
	if result != "testResource@testNamespace" {
		t.Errorf("attachNamespace was incorrect, got: %s, want: %s.", result, "testResource@testNamespace")
	}
}

// TestGetServiceAndNamespace function
func TestGetServiceAndNamespace(t *testing.T) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testService",
			Namespace: "testNamespace",
		},
	}
	result := getServiceAndNamespace(service)
	if result != "testService@testNamespace" {
		t.Errorf("getServiceAndNamespace was incorrect, got: %s, want: %s.", result, "testService@testNamespace")
	}
}

// TestExtractResourceAndNamespace function
func TestExtractResourceAndNamespace(t *testing.T) {
	// Test normal case
	name, namespace := extractResourceAndNamespace("testService@testNamespace")
	if name != "testService" || namespace != "testNamespace" {
		t.Errorf("extractResourceAndNamespace was incorrect, got: %s and %s, want: %s and %s.", name, namespace, "testService", "testNamespace")
	}

	// Test invalid case
	name, namespace = extractResourceAndNamespace("invalid")
	if name != "" || namespace != "" {
		t.Errorf("extractResourceAndNamespace was incorrect, got: %s and %s, want: %s and %s.", name, namespace, "", "")
	}
}

func TestExtractWorkloadNameFromRS(t *testing.T) {
	testCases := []struct {
		name           string
		replicaSetName string
		want           string
		shouldErr      bool
	}{
		{
			name:           "Valid ReplicaSet Name",
			replicaSetName: "my-deployment-5859ffc7ff",
			want:           "my-deployment",
			shouldErr:      false,
		},
		{
			name:           "Invalid ReplicaSet Name - No Hyphen",
			replicaSetName: "mydeployment5859ffc7ff",
			want:           "",
			shouldErr:      true,
		},
		{
			name:           "Invalid ReplicaSet Name - Less Than 10 Suffix Characters",
			replicaSetName: "my-deployment-bc2",
			want:           "",
			shouldErr:      true,
		},
		{
			name:           "Invalid ReplicaSet Name - More Than 10 Suffix Characters",
			replicaSetName: "my-deployment-5859ffc7ffx",
			want:           "",
			shouldErr:      true,
		},
		{
			name:           "Invalid ReplicaSet Name - Invalid Characters in Suffix",
			replicaSetName: "my-deployment-aeiou12345",
			want:           "",
			shouldErr:      true,
		},
		{
			name:           "Invalid ReplicaSet Name - Empty String",
			replicaSetName: "",
			want:           "",
			shouldErr:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractWorkloadNameFromRS(tc.replicaSetName)

			if (err != nil) != tc.shouldErr {
				t.Errorf("extractWorkloadNameFromRS() error = %v, wantErr %v", err, tc.shouldErr)
				return
			}

			if got != tc.want {
				t.Errorf("extractWorkloadNameFromRS() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestExtractWorkloadNameFromPodName(t *testing.T) {
	testCases := []struct {
		name      string
		podName   string
		want      string
		shouldErr bool
	}{
		{
			name:      "Valid Pod Name",
			podName:   "my-replicaset-bc24f",
			want:      "my-replicaset",
			shouldErr: false,
		},
		{
			name:      "Invalid Pod Name - No Hyphen",
			podName:   "myreplicasetbc24f",
			want:      "",
			shouldErr: true,
		},
		{
			name:      "Invalid Pod Name - Less Than 5 Suffix Characters",
			podName:   "my-replicaset-bc2",
			want:      "",
			shouldErr: true,
		},
		{
			name:      "Invalid Pod Name - More Than 5 Suffix Characters",
			podName:   "my-replicaset-bc24f5",
			want:      "",
			shouldErr: true,
		},
		{
			name:      "Invalid Pod Name - Empty String",
			podName:   "",
			want:      "",
			shouldErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractWorkloadNameFromPodName(tc.podName)

			if (err != nil) != tc.shouldErr {
				t.Errorf("extractWorkloadNameFromPodName() error = %v, wantErr %v", err, tc.shouldErr)
				return
			}

			if got != tc.want {
				t.Errorf("extractWorkloadNameFromPodName() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestGetWorkloadAndNamespace function
func TestGetWorkloadAndNamespace(t *testing.T) {
	// Test ReplicaSet case
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testPod",
			Namespace: "testNamespace",
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind: "ReplicaSet",
					Name: "testDeployment-5d68bc5f49",
				},
			},
		},
	}
	result := getWorkloadAndNamespace(pod)
	if result != "testDeployment@testNamespace" {
		t.Errorf("getDeploymentAndNamespace was incorrect, got: %s, want: %s.", result, "testDeployment@testNamespace")
	}

	// Test StatefulSet case
	pod.ObjectMeta.OwnerReferences[0].Kind = "StatefulSet"
	pod.ObjectMeta.OwnerReferences[0].Name = "testStatefulSet"
	result = getWorkloadAndNamespace(pod)
	if result != "testStatefulSet@testNamespace" {
		t.Errorf("getWorkloadAndNamespace was incorrect, got: %s, want: %s.", result, "testStatefulSet@testNamespace")
	}

	// Test Other case
	pod.ObjectMeta.OwnerReferences[0].Kind = "Other"
	pod.ObjectMeta.OwnerReferences[0].Name = "testOther"
	result = getWorkloadAndNamespace(pod)
	if result != "" {
		t.Errorf("getWorkloadAndNamespace was incorrect, got: %s, want: %s.", result, "")
	}

	// Test no OwnerReferences case
	pod.ObjectMeta.OwnerReferences = nil
	result = getWorkloadAndNamespace(pod)
	if result != "" {
		t.Errorf("getWorkloadAndNamespace was incorrect, got: %s, want: %s.", result, "")
	}
}

func TestServiceToWorkloadMapper_MapServiceToWorkload(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	serviceAndNamespaceToSelectors := &sync.Map{}
	workloadAndNamespaceToLabels := &sync.Map{}
	serviceToWorkload := &sync.Map{}

	serviceAndNamespaceToSelectors.Store("service1@namespace1", mapset.NewSet("label1=value1", "label2=value2"))
	workloadAndNamespaceToLabels.Store("deployment1@namespace1", mapset.NewSet("label1=value1", "label2=value2", "label3=value3"))

	mapper := NewServiceToWorkloadMapper(serviceAndNamespaceToSelectors, workloadAndNamespaceToLabels, serviceToWorkload, logger, mockDeleter)
	mapper.MapServiceToWorkload()

	if _, ok := serviceToWorkload.Load("service1@namespace1"); !ok {
		t.Errorf("Expected service1@namespace1 to be mapped to a workload, but it was not")
	}
}

func TestServiceToWorkloadMapper_MapServiceToWorkload_NoWorkload(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	serviceAndNamespaceToSelectors := &sync.Map{}
	workloadAndNamespaceToLabels := &sync.Map{}
	serviceToWorkload := &sync.Map{}

	// Add a service with no matching workload
	serviceAndNamespace := "service@namespace"
	serviceAndNamespaceToSelectors.Store(serviceAndNamespace, mapset.NewSet("label1=value1"))
	serviceToWorkload.Store(serviceAndNamespace, "workload@namespace")

	mapper := NewServiceToWorkloadMapper(serviceAndNamespaceToSelectors, workloadAndNamespaceToLabels, serviceToWorkload, logger, mockDeleter)
	mapper.MapServiceToWorkload()

	// Check that the service was deleted from serviceToWorkload
	if _, ok := serviceToWorkload.Load(serviceAndNamespace); ok {
		t.Errorf("Service was not deleted from serviceToWorkload")
	}
}

func TestServiceToWorkloadMapper_MapServiceToWorkload_MultipleWorkloads(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	serviceAndNamespaceToSelectors := &sync.Map{}
	workloadAndNamespaceToLabels := &sync.Map{}
	serviceToWorkload := &sync.Map{}

	serviceAndNamespace := "service@namespace"
	serviceAndNamespaceToSelectors.Store(serviceAndNamespace, mapset.NewSet("label1=value1", "label2=value2"))

	// Add two workloads with matching labels to the service
	workloadAndNamespaceToLabels.Store("workload1@namespace", mapset.NewSet("label1=value1", "label2=value2", "label3=value3"))
	workloadAndNamespaceToLabels.Store("workload2@namespace", mapset.NewSet("label1=value1", "label2=value2", "label4=value4"))

	mapper := NewServiceToWorkloadMapper(serviceAndNamespaceToSelectors, workloadAndNamespaceToLabels, serviceToWorkload, logger, mockDeleter)
	mapper.MapServiceToWorkload()

	// Check that the service does not map to any workload
	if _, ok := serviceToWorkload.Load(serviceAndNamespace); ok {
		t.Errorf("Unexpected mapping of service to multiple workloads")
	}
}

func TestMapServiceToWorkload_StopsWhenSignaled(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	serviceAndNamespaceToSelectors := &sync.Map{}
	workloadAndNamespaceToLabels := &sync.Map{}
	serviceToWorkload := &sync.Map{}

	stopchan := make(chan struct{})

	// Signal the stopchan to stop after 100 milliseconds
	time.AfterFunc(100*time.Millisecond, func() {
		close(stopchan)
	})

	mapper := NewServiceToWorkloadMapper(serviceAndNamespaceToSelectors, workloadAndNamespaceToLabels, serviceToWorkload, logger, mockDeleter)

	start := time.Now()
	mapper.Start(stopchan)
	duration := time.Since(start)

	// Check that the function stopped in a reasonable time after the stop signal
	if duration > 200*time.Millisecond {
		t.Errorf("mapServiceToWorkload did not stop in a reasonable time after the stop signal, duration: %v", duration)
	}
}

func TestOnAddOrUpdateService(t *testing.T) {
	// Create a fake service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myservice",
			Namespace: "mynamespace",
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "1.2.3.4",
			Selector: map[string]string{
				"app": "myapp",
			},
		},
	}

	// Create the maps
	ipToServiceAndNamespace := &sync.Map{}
	serviceAndNamespaceToSelectors := &sync.Map{}

	// Call the function
	onAddOrUpdateService(service, ipToServiceAndNamespace, serviceAndNamespaceToSelectors)

	// Check that the maps contain the expected entries
	if _, ok := ipToServiceAndNamespace.Load("1.2.3.4"); !ok {
		t.Errorf("ipToServiceAndNamespace does not contain the service IP")
	}
	if _, ok := serviceAndNamespaceToSelectors.Load("myservice@mynamespace"); !ok {
		t.Errorf("serviceAndNamespaceToSelectors does not contain the service")
	}
}

func TestOnDeleteService(t *testing.T) {
	// Create a fake service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myservice",
			Namespace: "mynamespace",
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "1.2.3.4",
			Selector: map[string]string{
				"app": "myapp",
			},
		},
	}

	// Create the maps and add the service to them
	ipToServiceAndNamespace := &sync.Map{}
	ipToServiceAndNamespace.Store("1.2.3.4", "myservice@mynamespace")
	serviceAndNamespaceToSelectors := &sync.Map{}
	serviceAndNamespaceToSelectors.Store("myservice@mynamespace", mapset.NewSet("app=myapp"))

	// Call the function
	onDeleteService(service, ipToServiceAndNamespace, serviceAndNamespaceToSelectors, mockDeleter)

	// Check that the maps do not contain the service
	if _, ok := ipToServiceAndNamespace.Load("1.2.3.4"); ok {
		t.Errorf("ipToServiceAndNamespace still contains the service IP")
	}
	if _, ok := serviceAndNamespaceToSelectors.Load("myservice@mynamespace"); ok {
		t.Errorf("serviceAndNamespaceToSelectors still contains the service")
	}
}

func TestOnAddOrUpdatePod(t *testing.T) {
	logger, _ := zap.NewProduction()

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

		onAddOrUpdatePod(pod, nil, ipToPod, podToWorkloadAndNamespace, workloadAndNamespaceToLabels, workloadPodCount, true, logger, mockDeleter)

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

		onAddOrUpdatePod(pod, nil, ipToPod, podToWorkloadAndNamespace, workloadAndNamespaceToLabels, workloadPodCount, true, logger, mockDeleter)

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
		workloadPodCount := map[string]int{}

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
		onAddOrUpdatePod(pod, nil, ipToPod, podToWorkloadAndNamespace, workloadAndNamespaceToLabels, workloadPodCount, true, logger, mockDeleter)

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
		}

		// add the pod
		onAddOrUpdatePod(pod2, pod, ipToPod, podToWorkloadAndNamespace, workloadAndNamespaceToLabels, workloadPodCount, false, logger, mockDeleter)

		// Test the mappings in ipToPod
		if _, ok := ipToPod.Load("5.6.7.8:8080"); ok {
			t.Errorf("ipToPod[%s] should be deleted", "5.6.7.8:8080")
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
	logger, _ := zap.NewProduction()

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

		onDeletePod(pod, ipToPod, podToWorkloadAndNamespace, workloadAndNamespaceToLabels, workloadPodCount, logger, mockDeleter)

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

		onDeletePod(pod, ipToPod, podToWorkloadAndNamespace, workloadAndNamespaceToLabels, workloadPodCount, logger, mockDeleter)

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

func TestEksResolver(t *testing.T) {
	logger, _ := zap.NewProduction()
	ctx := context.Background()

	t.Run("Test GetWorkloadAndNamespaceByIP", func(t *testing.T) {
		resolver := &kubernetesResolver{
			logger:                    logger,
			clusterName:               "test",
			ipToPod:                   &sync.Map{},
			podToWorkloadAndNamespace: &sync.Map{},
			ipToServiceAndNamespace:   &sync.Map{},
			serviceToWorkload:         &sync.Map{},
		}

		ip := "1.2.3.4"
		pod := "testPod"
		workloadAndNamespace := "testDeployment@testNamespace"

		// Pre-fill the resolver maps
		resolver.ipToPod.Store(ip, pod)
		resolver.podToWorkloadAndNamespace.Store(pod, workloadAndNamespace)

		// Test existing IP
		workload, namespace, err := resolver.GetWorkloadAndNamespaceByIP(ip)
		if err != nil || workload != "testDeployment" || namespace != "testNamespace" {
			t.Errorf("Expected testDeployment@testNamespace, got %s@%s, error: %v", workload, namespace, err)
		}

		// Test non-existing IP
		_, _, err = resolver.GetWorkloadAndNamespaceByIP("5.6.7.8")
		if err == nil || !strings.Contains(err.Error(), "no kubernetes workload found for ip: 5.6.7.8") {
			t.Errorf("Expected error, got %v", err)
		}

		// Test ip in ipToServiceAndNamespace but not in ipToPod
		newIP := "2.3.4.5"
		serviceAndNamespace := "testService@testNamespace"
		resolver.ipToServiceAndNamespace.Store(newIP, serviceAndNamespace)
		resolver.serviceToWorkload.Store(serviceAndNamespace, workloadAndNamespace)
		workload, namespace, err = resolver.GetWorkloadAndNamespaceByIP(newIP)
		if err != nil || workload != "testDeployment" || namespace != "testNamespace" {
			t.Errorf("Expected testDeployment@testNamespace, got %s@%s, error: %v", workload, namespace, err)
		}
	})

	t.Run("Test Stop", func(t *testing.T) {
		resolver := &kubernetesResolver{
			logger:     logger,
			safeStopCh: &safeChannel{ch: make(chan struct{}), closed: false},
		}

		err := resolver.Stop(ctx)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if !resolver.safeStopCh.closed {
			t.Errorf("Expected channel to be closed")
		}

		// Test closing again
		err = resolver.Stop(ctx)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("Test Process", func(t *testing.T) {
		// helper function to get string values from the attributes
		getStrAttr := func(attributes pcommon.Map, key string, t *testing.T) string {
			if value, ok := attributes.Get(key); ok {
				return value.AsString()
			} else {
				t.Errorf("Failed to get value for key: %s", key)
				return ""
			}
		}

		logger, _ := zap.NewProduction()
		resolver := &kubernetesResolver{
			logger:                    logger,
			clusterName:               "test",
			platformCode:              config.PlatformEKS,
			ipToPod:                   &sync.Map{},
			podToWorkloadAndNamespace: &sync.Map{},
			ipToServiceAndNamespace:   &sync.Map{},
			serviceToWorkload:         &sync.Map{},
		}

		// Test case 1: "aws.remote.service" contains IP:Port
		attributes := pcommon.NewMap()
		attributes.PutStr(attr.AWSRemoteService, "192.0.2.1:8080")
		resourceAttributes := pcommon.NewMap()
		resolver.ipToPod.Store("192.0.2.1:8080", "test-pod")
		resolver.podToWorkloadAndNamespace.Store("test-pod", "test-deployment@test-namespace")
		err := resolver.Process(attributes, resourceAttributes)
		assert.NoError(t, err)
		assert.Equal(t, "test-deployment", getStrAttr(attributes, attr.AWSRemoteService, t))
		assert.Equal(t, "eks:test/test-namespace", getStrAttr(attributes, attr.AWSRemoteEnvironment, t))

		// Test case 2: "aws.remote.service" contains only IP
		attributes = pcommon.NewMap()
		attributes.PutStr(attr.AWSRemoteService, "192.0.2.2")
		resourceAttributes = pcommon.NewMap()
		resolver.ipToPod.Store("192.0.2.2", "test-pod-2")
		resolver.podToWorkloadAndNamespace.Store("test-pod-2", "test-deployment-2@test-namespace-2")
		err = resolver.Process(attributes, resourceAttributes)
		assert.NoError(t, err)
		assert.Equal(t, "test-deployment-2", getStrAttr(attributes, attr.AWSRemoteService, t))
		assert.Equal(t, "eks:test/test-namespace-2", getStrAttr(attributes, attr.AWSRemoteEnvironment, t))

		// Test case 3: "aws.remote.service" contains non-ip string
		attributes = pcommon.NewMap()
		attributes.PutStr(attr.AWSRemoteService, "not-an-ip")
		resourceAttributes = pcommon.NewMap()
		err = resolver.Process(attributes, resourceAttributes)
		assert.NoError(t, err)
		assert.Equal(t, "not-an-ip", getStrAttr(attributes, attr.AWSRemoteService, t))

		// Test case 4: Process with valid IP but GetWorkloadAndNamespaceByIP returns error
		attributes = pcommon.NewMap()
		attributes.PutStr(attr.AWSRemoteService, "192.168.1.2")
		resourceAttributes = pcommon.NewMap()
		err = resolver.Process(attributes, resourceAttributes)
		assert.NoError(t, err)
		assert.Equal(t, "UnknownRemoteService", getStrAttr(attributes, attr.AWSRemoteService, t))
	})
}

func TestK8sResourceAttributesResolverOnEKS(t *testing.T) {
	eksdetector.NewDetector = eksdetector.TestEKSDetector
	eksdetector.IsEKS = eksdetector.TestIsEKSCacheEKS
	// helper function to get string values from the attributes
	getStrAttr := func(attributes pcommon.Map, key string, t *testing.T) string {
		if value, ok := attributes.Get(key); ok {
			return value.AsString()
		} else {
			t.Errorf("Failed to get value for key: %s", key)
			return ""
		}
	}

	resolver := newKubernetesResourceAttributesResolver(config.PlatformEKS, "test-cluster")

	resourceAttributesBase := map[string]string{
		"cloud.provider":                    "aws",
		"k8s.namespace.name":                "test-namespace-3",
		"host.id":                           "instance-id",
		"host.name":                         "hostname",
		"ec2.tag.aws:autoscaling:groupName": "asg",
	}

	tests := []struct {
		name                        string
		resourceAttributesOverwrite map[string]string
		expectedAttributes          map[string]string
	}{
		{
			"testDefault",
			map[string]string{},

			map[string]string{
				attr.AWSLocalEnvironment:            "eks:test-cluster/test-namespace-3",
				common.AttributeK8SNamespace:        "test-namespace-3",
				common.AttributeEKSClusterName:      "test-cluster",
				common.AttributeEC2InstanceId:       "instance-id",
				common.AttributeHost:                "hostname",
				common.AttributeEC2AutoScalingGroup: "asg",
			},
		},
		{
			"testOverwrite",
			map[string]string{
				semconv.AttributeDeploymentEnvironment: "custom-env",
			},
			map[string]string{
				attr.AWSLocalEnvironment:            "custom-env",
				common.AttributeK8SNamespace:        "test-namespace-3",
				common.AttributeEKSClusterName:      "test-cluster",
				common.AttributeEC2InstanceId:       "instance-id",
				common.AttributeHost:                "hostname",
				common.AttributeEC2AutoScalingGroup: "asg",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attributes := pcommon.NewMap()
			resourceAttributes := pcommon.NewMap()
			for key, val := range resourceAttributesBase {
				resourceAttributes.PutStr(key, val)
			}
			for key, val := range tt.resourceAttributesOverwrite {
				resourceAttributes.PutStr(key, val)
			}
			err := resolver.Process(attributes, resourceAttributes)
			assert.NoError(t, err)

			for key, val := range tt.expectedAttributes {
				assert.Equal(t, val, getStrAttr(attributes, key, t), fmt.Sprintf("expected %s for key %s", val, key))
			}
			assert.Equal(t, "/aws/containerinsights/test-cluster/application", getStrAttr(resourceAttributes, semconv.AttributeAWSLogGroupNames, t))
		})
	}
}

func TestK8sResourceAttributesResolverOnK8S(t *testing.T) {
	eksdetector.NewDetector = eksdetector.TestK8sDetector
	eksdetector.IsEKS = eksdetector.TestIsEKSCacheK8s
	// helper function to get string values from the attributes
	getStrAttr := func(attributes pcommon.Map, key string, t *testing.T) string {
		if value, ok := attributes.Get(key); ok {
			return value.AsString()
		} else {
			t.Errorf("Failed to get value for key: %s", key)
			return ""
		}
	}

	resolver := newKubernetesResourceAttributesResolver(config.PlatformK8s, "test-cluster")

	resourceAttributesBase := map[string]string{
		"cloud.provider":                    "aws",
		"k8s.namespace.name":                "test-namespace-3",
		"host.id":                           "instance-id",
		"host.name":                         "hostname",
		"ec2.tag.aws:autoscaling:groupName": "asg",
	}

	tests := []struct {
		name                        string
		resourceAttributesOverwrite map[string]string
		expectedAttributes          map[string]string
	}{
		{
			"testDefaultOnK8s",
			map[string]string{},

			map[string]string{
				attr.AWSLocalEnvironment:            "k8s:test-cluster/test-namespace-3",
				common.AttributeK8SNamespace:        "test-namespace-3",
				common.AttributeK8SClusterName:      "test-cluster",
				common.AttributeEC2InstanceId:       "instance-id",
				common.AttributeHost:                "hostname",
				common.AttributeEC2AutoScalingGroup: "asg",
			},
		},
		{
			"testOverwriteOnK8s",
			map[string]string{
				semconv.AttributeDeploymentEnvironment: "custom-env",
			},
			map[string]string{
				attr.AWSLocalEnvironment:            "custom-env",
				common.AttributeK8SNamespace:        "test-namespace-3",
				common.AttributeK8SClusterName:      "test-cluster",
				common.AttributeEC2InstanceId:       "instance-id",
				common.AttributeHost:                "hostname",
				common.AttributeEC2AutoScalingGroup: "asg",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attributes := pcommon.NewMap()
			resourceAttributes := pcommon.NewMap()
			for key, val := range resourceAttributesBase {
				resourceAttributes.PutStr(key, val)
			}
			for key, val := range tt.resourceAttributesOverwrite {
				resourceAttributes.PutStr(key, val)
			}
			err := resolver.Process(attributes, resourceAttributes)
			assert.NoError(t, err)

			for key, val := range tt.expectedAttributes {
				assert.Equal(t, val, getStrAttr(attributes, key, t), fmt.Sprintf("expected %s for key %s", val, key))
			}
			assert.Equal(t, "/aws/containerinsights/test-cluster/application", getStrAttr(resourceAttributes, semconv.AttributeAWSLogGroupNames, t))
		})
	}
}

func TestK8sResourceAttributesResolverOnK8SOnPrem(t *testing.T) {
	eksdetector.NewDetector = eksdetector.TestK8sDetector
	// helper function to get string values from the attributes
	getStrAttr := func(attributes pcommon.Map, key string, t *testing.T) string {
		if value, ok := attributes.Get(key); ok {
			return value.AsString()
		} else {
			t.Errorf("Failed to get value for key: %s", key)
			return ""
		}
	}

	resolver := newKubernetesResourceAttributesResolver(config.PlatformK8s, "test-cluster")

	resourceAttributesBase := map[string]string{
		"cloud.provider":     "aws",
		"k8s.namespace.name": "test-namespace-3",
		"host.name":          "hostname",
	}

	tests := []struct {
		name                        string
		resourceAttributesOverwrite map[string]string
		expectedAttributes          map[string]string
	}{
		{
			"testDefault",
			map[string]string{},

			map[string]string{
				attr.AWSLocalEnvironment:       "k8s:test-cluster/test-namespace-3",
				common.AttributeK8SNamespace:   "test-namespace-3",
				common.AttributeK8SClusterName: "test-cluster",
				common.AttributeHost:           "hostname",
			},
		},
		{
			"testOverwrite",
			map[string]string{
				semconv.AttributeDeploymentEnvironment: "custom-env",
			},
			map[string]string{
				attr.AWSLocalEnvironment:       "custom-env",
				common.AttributeK8SNamespace:   "test-namespace-3",
				common.AttributeK8SClusterName: "test-cluster",
				common.AttributeHost:           "hostname",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attributes := pcommon.NewMap()
			resourceAttributes := pcommon.NewMap()
			for key, val := range resourceAttributesBase {
				resourceAttributes.PutStr(key, val)
			}
			for key, val := range tt.resourceAttributesOverwrite {
				resourceAttributes.PutStr(key, val)
			}
			err := resolver.Process(attributes, resourceAttributes)
			assert.NoError(t, err)

			for key, val := range tt.expectedAttributes {
				assert.Equal(t, val, getStrAttr(attributes, key, t), fmt.Sprintf("expected %s for key %s", val, key))
			}
			assert.Equal(t, "/aws/containerinsights/test-cluster/application", getStrAttr(resourceAttributes, semconv.AttributeAWSLogGroupNames, t))

			// EC2 related fields that should not exist for on-prem
			_, exists := attributes.Get(common.AttributeEC2AutoScalingGroup)
			assert.False(t, exists)

			_, exists = attributes.Get(common.AttributeEC2InstanceId)
			assert.False(t, exists)
		})
	}
}

func TestExtractIPPort(t *testing.T) {
	// Test valid IP:Port
	ip, port, ok := extractIPPort("192.0.2.0:8080")
	assert.Equal(t, "192.0.2.0", ip)
	assert.Equal(t, "8080", port)
	assert.True(t, ok)

	// Test invalid IP:Port
	ip, port, ok = extractIPPort("192.0.2:8080")
	assert.Equal(t, "", ip)
	assert.Equal(t, "", port)
	assert.False(t, ok)

	// Test IP only
	ip, port, ok = extractIPPort("192.0.2.0")
	assert.Equal(t, "", ip)
	assert.Equal(t, "", port)
	assert.False(t, ok)
}

func TestGetHostNetworkPorts(t *testing.T) {
	// Test Pod with no ports
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{},
			},
		},
	}
	assert.Empty(t, getHostNetworkPorts(pod))

	// Test Pod with one port
	pod = &corev1.Pod{
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
	assert.Equal(t, []string{"8080"}, getHostNetworkPorts(pod))

	// Test Pod with multiple ports
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
	}
	assert.Equal(t, []string{"8080", "8081"}, getHostNetworkPorts(pod))
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
			name: "Old and New Pod Use Host Network, Different Ports",
			oldPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mypod",
				},
				Status: corev1.PodStatus{
					HostIP: "192.168.1.1",
				},
				Spec: corev1.PodSpec{
					HostNetwork: true,
					Containers: []corev1.Container{
						{
							Ports: []corev1.ContainerPort{
								{
									HostPort: 8000,
								},
							},
						},
					},
				},
			},
			newPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mypod",
				},
				Status: corev1.PodStatus{
					HostIP: "192.168.1.1",
				},
				Spec: corev1.PodSpec{
					HostNetwork: true,
					Containers: []corev1.Container{
						{
							Ports: []corev1.ContainerPort{
								{
									HostPort: 8080,
								},
							},
						},
					},
				},
			},
			initialIPToPod: map[string]string{
				"192.168.1.1:8000": "mypod",
			},
			expectedIPToPod: map[string]string{
				"192.168.1.1:8080": "mypod",
			},
		},
		// ...other test cases...
		{
			name: "Old Pod Uses Host Network, New Pod Does Not",
			oldPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mypod",
				},
				Status: corev1.PodStatus{
					HostIP: "192.168.1.2",
				},
				Spec: corev1.PodSpec{
					HostNetwork: true,
					Containers: []corev1.Container{
						{
							Ports: []corev1.ContainerPort{
								{
									HostPort: 8001,
								},
							},
						},
					},
				},
			},
			newPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mypod",
				},
				Status: corev1.PodStatus{
					PodIP: "10.0.0.1",
				},
				Spec: corev1.PodSpec{
					HostNetwork: false,
				},
			},
			initialIPToPod: map[string]string{
				"192.168.1.2:8001": "mypod",
			},
			expectedIPToPod: map[string]string{
				"10.0.0.1": "mypod",
			},
		},
		{
			name: "Old Pod Does Not Use Host Network, New Pod Does",
			oldPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mypod",
				},
				Status: corev1.PodStatus{
					PodIP: "10.0.0.2",
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
					HostIP: "192.168.1.3",
				},
				Spec: corev1.PodSpec{
					HostNetwork: true,
					Containers: []corev1.Container{
						{
							Ports: []corev1.ContainerPort{
								{
									HostPort: 8002,
								},
							},
						},
					},
				},
			},
			initialIPToPod: map[string]string{
				"10.0.0.2": "mypod",
			},
			expectedIPToPod: map[string]string{
				"192.168.1.3:8002": "mypod",
			},
		},
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
			handlePodUpdate(tc.newPod, tc.oldPod, ipToPod, mockDeleter)

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
