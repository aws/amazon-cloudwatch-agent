// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package eksdetector

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func TestNewDetector(t *testing.T) {
	getInClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{}, nil
	}

	testDetector1, err := NewDetector()
	assert.NoError(t, err)
	assert.NotNil(t, testDetector1)

	getInClusterConfig = func() (*rest.Config, error) {
		return nil, fmt.Errorf("error")
	}
	_, err = NewDetector()
	assert.Error(t, err)
}

func TestIsEKSSingleton(t *testing.T) {
	getInClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{}, nil
	}

	NewDetector = TestEKSDetector
	value1 := IsEKS()
	assert.NoError(t, value1.Err)
	value2 := IsEKS()
	assert.NoError(t, value2.Err)

	assert.True(t, value1 == value2)
}

// Tests EKS resource detector running in EKS environment
func TestEKS(t *testing.T) {
	testDetector := new(MockDetector)
	NewDetector = func() (Detector, error) {
		return testDetector, nil
	}

	testDetector.On("getConfigMap", authConfigNamespace, authConfigConfigMap).Return(map[string]string{conventions.AttributeK8SClusterName: "my-cluster"}, nil)
	isEks := IsEKS()
	assert.True(t, isEks.Value)
	assert.NoError(t, isEks.Err)
}

func Test_getConfigMap(t *testing.T) {
	// No matching configmap
	client := fake.NewSimpleClientset()
	testDetector := &EksDetector{Clientset: client}
	res, err := testDetector.getConfigMap("test", "test")
	assert.Error(t, err)
	assert.Nil(t, res)

	// matching configmap
	cm := &v1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{Kind: "ConfigMap", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Namespace: authConfigNamespace, Name: authConfigConfigMap},
		Data:       make(map[string]string),
	}

	client = fake.NewSimpleClientset(cm)
	testDetector = &EksDetector{Clientset: client}

	res, err = testDetector.getConfigMap(authConfigNamespace, authConfigConfigMap)
	assert.NoError(t, err)
	assert.NotNil(t, res)
}

func Test_getWorkloadType_EnvNotSet(t *testing.T) {
	client := fake.NewSimpleClientset()
	testDetector := &EksDetector{Clientset: client}

	workloadType, err := testDetector.getWorkloadType()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "POD_NAME/K8S_NAMESPACE environment variables not set")
	assert.Equal(t, "", workloadType)
}

func Test_getWorkloadType_PodNotFound(t *testing.T) {
	os.Setenv("POD_NAME", "nonexistent-pod")
	os.Setenv("K8S_NAMESPACE", "default")

	client := fake.NewSimpleClientset()
	testDetector := &EksDetector{Clientset: client}

	workloadType, err := testDetector.getWorkloadType()
	assert.Error(t, err)
	assert.Equal(t, "", workloadType)
}

func Test_getWorkloadType_DaemonSet(t *testing.T) {
	podName := "test-pod"
	namespace := "default"
	os.Setenv("POD_NAME", podName)
	os.Setenv("K8S_NAMESPACE", namespace)

	// Create a pod with an OwnerReference of kind DaemonSet
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind: "DaemonSet",
					Name: "test-daemonset",
				},
			},
		},
	}

	client := fake.NewSimpleClientset(pod)
	testDetector := &EksDetector{Clientset: client}

	workloadType, err := testDetector.getWorkloadType()
	assert.NoError(t, err)
	assert.Equal(t, "DaemonSet", workloadType)
}

func Test_getWorkloadType_StatefulSet(t *testing.T) {
	podName := "test-pod"
	namespace := "default"
	os.Setenv("POD_NAME", podName)
	os.Setenv("K8S_NAMESPACE", namespace)

	// Create a pod with an OwnerReference of kind StatefulSet
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind: "StatefulSet",
					Name: "test-statefulset",
				},
			},
		},
	}

	client := fake.NewSimpleClientset(pod)
	testDetector := &EksDetector{Clientset: client}

	workloadType, err := testDetector.getWorkloadType()
	assert.NoError(t, err)
	assert.Equal(t, "StatefulSet", workloadType)
}

func Test_getWorkloadType_ReplicaSet(t *testing.T) {
	podName := "test-pod"
	namespace := "default"
	os.Setenv("POD_NAME", podName)
	os.Setenv("K8S_NAMESPACE", namespace)

	// Create a pod with an OwnerReference of kind ReplicaSet
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind: "ReplicaSet",
					Name: "test-replicaset",
				},
			},
		},
	}

	client := fake.NewSimpleClientset(pod)
	testDetector := &EksDetector{Clientset: client}

	workloadType, err := testDetector.getWorkloadType()
	assert.NoError(t, err)
	assert.Equal(t, "Deployment", workloadType)
}

func Test_getWorkloadType_Unknown(t *testing.T) {
	podName := "test-pod"
	namespace := "default"
	os.Setenv("POD_NAME", podName)
	os.Setenv("K8S_NAMESPACE", namespace)

	// Create a pod with no OwnerReferences
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
		},
	}

	client := fake.NewSimpleClientset(pod)
	testDetector := &EksDetector{Clientset: client}

	workloadType, err := testDetector.getWorkloadType()
	assert.NoError(t, err)
	assert.Equal(t, "Unknown", workloadType)
}

func Test_getClientError(t *testing.T) {
	//InClusterConfig error
	getInClusterConfig = func() (*rest.Config, error) {
		return nil, fmt.Errorf("test error")
	}

	_, err := getClient()
	assert.Error(t, err)

	//Getting Kubernetes client error
	getInClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{}, nil
	}
	getKubernetesClient = func(confs *rest.Config) (kubernetes.Interface, error) {
		return nil, fmt.Errorf("test error")
	}

	_, err = getClient()
	assert.Error(t, err)
}
