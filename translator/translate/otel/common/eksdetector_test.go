// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

type MockDetector struct {
	mock.Mock
}

func (detector *MockDetector) getConfigMap(namespace string, name string) (map[string]string, error) {
	args := detector.Called(namespace, name)
	return args.Get(0).(map[string]string), args.Error(1)
}

func TestNewDetector(t *testing.T) {
	getInClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{}, nil
	}

	testDetector1, err := NewDetector()
	assert.NoError(t, err)
	assert.NotNil(t, testDetector1)

	// Test singleton
	testDetector2, err := NewDetector()
	assert.NoError(t, err)
	assert.True(t, testDetector1 == testDetector2)
}

// Tests EKS resource detector running in EKS environment
func TestEKS(t *testing.T) {
	testDetector := new(MockDetector)
	NewDetector = func() (Detector, error) {
		return testDetector, nil
	}

	testDetector.On("getConfigMap", authConfigNamespace, authConfigConfigMap).Return(map[string]string{conventions.AttributeK8SClusterName: "my-cluster"}, nil)
	isEks, err := IsEKS()
	assert.True(t, isEks)
	assert.NoError(t, err)
}

// Tests EKS resource detector not running in EKS environment by verifying resource is not running on k8s
func TestNotEKS(t *testing.T) {
	testDetector := new(MockDetector)

	// Detector creation failure
	NewDetector = func() (Detector, error) {
		return nil, fmt.Errorf("test error")
	}
	isEks, err := IsEKS()
	assert.False(t, isEks)
	assert.Error(t, err)

	//get configmap failure
	NewDetector = func() (Detector, error) {
		return testDetector, nil
	}

	testDetector.On("getConfigMap", authConfigNamespace, authConfigConfigMap).Return(map[string]string{}, fmt.Errorf("error"))
	isEks, err = IsEKS()
	assert.False(t, isEks)
	assert.Error(t, err)
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
