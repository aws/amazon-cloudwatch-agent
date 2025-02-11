// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package eksdetector

import (
	"encoding/base64"
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
		return &rest.Config{BearerToken: "header.payload.signature"}, nil
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
		return &rest.Config{BearerToken: "header.payload.signature"}, nil
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

	testDetector.On("getIssuer").Return("https://oidc.eks.us-west-2.amazonaws.com/id/someid", nil)
	isEks := IsEKS()
	assert.True(t, isEks.Value)
	assert.NoError(t, isEks.Err)
}

func Test_getIssuer(t *testing.T) {
	resetTestState()
	client := fake.NewSimpleClientset()
	testDetector := &EksDetector{Clientset: client}

	payload := `{"iss":"https://oidc.eks.us-west-2.amazonaws.com/id/someid"}`
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	dummyToken := "header." + encodedPayload + ".signature"

	getInClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{BearerToken: dummyToken}, nil
	}

	issuer, err := testDetector.getIssuer()
	assert.NoError(t, err)
	assert.Equal(t, "https://oidc.eks.us-west-2.amazonaws.com/id/someid", issuer)
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
		return &rest.Config{BearerToken: "header.payload.signature"}, nil
	}
	getKubernetesClient = func(confs *rest.Config) (kubernetes.Interface, error) {
		return nil, fmt.Errorf("test error")
	}

	_, err = getClient()
	assert.Error(t, err)
}
