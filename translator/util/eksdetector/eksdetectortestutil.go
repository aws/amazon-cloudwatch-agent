// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package eksdetector

import (
	"github.com/stretchr/testify/mock"
	"k8s.io/client-go/kubernetes/fake"
)

var (
	// TestEKSDetector is used for unit testing EKS route
	TestEKSDetector = func() (Detector, error) {
		return &EksDetector{Clientset: fake.NewSimpleClientset()}, nil
	}
	// TestK8sDetector is used for unit testing k8s route
	TestK8sDetector = func() (Detector, error) {
		return &EksDetector{Clientset: fake.NewSimpleClientset()}, nil
	}

	// TestIsEKSCacheEKS is used for unit testing EKS route
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

func (detector *MockDetector) getIssuer() (string, error) {
	args := detector.Called()
	return args.Get(0).(string), args.Error(1)
}
