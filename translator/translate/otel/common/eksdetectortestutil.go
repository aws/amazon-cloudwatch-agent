// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common

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
)

type MockDetector struct {
	mock.Mock
}

func (detector *MockDetector) getConfigMap(namespace string, name string) (map[string]string, error) {
	args := detector.Called(namespace, name)
	return args.Get(0).(map[string]string), args.Error(1)
}
