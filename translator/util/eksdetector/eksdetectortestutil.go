// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package eksdetector

import (
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var (
	// TestEKSDetector is used for unit testing EKS route
	TestEKSDetector = func() (Detector, error) {
		cm := &v1.ConfigMap{
			TypeMeta:   metav1.TypeMeta{Kind: "ConfigMap", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system", Name: "aws-auth"},
			Data:       make(map[string]string),
		}
		return &EksDetector{Clientset: fake.NewSimpleClientset(cm)}, nil
	}
	// TestK8sDetector is used for unit testing k8s route
	TestK8sDetector = func() (Detector, error) {
		return &EksDetector{Clientset: fake.NewSimpleClientset()}, nil
	}

	// TestIsEKSCacheEKS os used for unit testing EKS route
	TestIsEKSCacheEKS = func() IsEKSCache {
		return IsEKSCache{Value: true, Err: nil}
	}

	// TestIsEKSCacheK8s is used for unit testing K8s route
	TestIsEKSCacheK8s = func() IsEKSCache {
		return IsEKSCache{Value: false, Err: nil}
	}
)

type MockDetector struct {
	mock.Mock
}

func (detector *MockDetector) getConfigMap(namespace string, name string) (map[string]string, error) {
	args := detector.Called(namespace, name)
	return args.Get(0).(map[string]string), args.Error(1)
}
