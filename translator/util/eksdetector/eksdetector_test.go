// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package eksdetector

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func resetTestState() {
	once = sync.Once{}
	isEKSCacheSingleton = IsEKSCache{}
}

func TestNewDetector(t *testing.T) {
	resetTestState()

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
	resetTestState()

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
	resetTestState()
	testDetector := new(MockDetector)
	NewDetector = func() (Detector, error) {
		return testDetector, nil
	}

	testDetector.On("getServerVersion").Return("v1.23-eks", nil)
	isEks := IsEKS()
	assert.True(t, isEks.Value)
	assert.NoError(t, isEks.Err)
}

func Test_getServerVersion(t *testing.T) {
	resetTestState()
	client := fake.NewSimpleClientset()
	testDetector := &EksDetector{Clientset: client}
	res, err := testDetector.getServerVersion()
	assert.NoError(t, err)
	assert.NotEmpty(t, res)
}

func Test_getClientError(t *testing.T) {
	resetTestState()

	//InClusterConfig error
	getInClusterConfig = func() (*rest.Config, error) {
		return nil, fmt.Errorf("test error")
	}

	_, err := getClient()
	assert.Error(t, err)
	resetTestState()

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
