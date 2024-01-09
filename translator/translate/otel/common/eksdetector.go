// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"context"
	"fmt"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Detector interface {
	getConfigMap(namespace string, name string) (map[string]string, error)
}

type EksDetector struct {
	Clientset kubernetes.Interface
}

const (
	authConfigNamespace = "kube-system"
	authConfigConfigMap = "aws-auth"
)

var _ Detector = (*EksDetector)(nil)

var (
	detectorSingleton Detector
	once              sync.Once
)

var (
	getInClusterConfig  = func() (*rest.Config, error) { return rest.InClusterConfig() }
	getKubernetesClient = func(confs *rest.Config) (kubernetes.Interface, error) { return kubernetes.NewForConfig(confs) }
	// NewDetector creates a new singleton detector for EKS
	NewDetector = func() (Detector, error) {
		var errors error
		once.Do(func() {
			if clientset, err := getClient(); err != nil {
				errors = err
			} else {
				detectorSingleton = &EksDetector{Clientset: clientset}
			}
		})

		return detectorSingleton, errors
	}
)

// IsEKS checks if the agent is running on EKS. This is done by using the kubernetes API to determine if the aws-auth
// configmap exists in the kube-system namespace
func IsEKS(eksDetector Detector) bool {

	// Make HTTP GET request
	awsAuth, err := eksDetector.getConfigMap(authConfigNamespace, authConfigConfigMap)
	if err != nil {
		return false
	}

	return awsAuth != nil
}

// getConfigMap retrieves the configmap with the provided name in the provided namespace
func (d *EksDetector) getConfigMap(namespace string, name string) (map[string]string, error) {
	configMap, err := d.Clientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve ConfigMap %s/%s: %w", namespace, name, err)
	}

	return configMap.Data, nil
}

func getClient() (kubernetes.Interface, error) {
	//Get cluster config
	confs, err := getInClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create config: %w", err)
	}

	// Create Clientset using generated configuration
	clientset, err := getKubernetesClient(confs)
	if err != nil {
		return nil, fmt.Errorf("failed to create Clientset for Kubernetes client")
	}

	return clientset, err
}
