// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package eksdetector

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

type IsEKSCache struct {
	Value bool
	Err   error
}

const (
	authConfigNamespace = "kube-system"
	authConfigConfigMap = "aws-auth"
)

var _ Detector = (*EksDetector)(nil)

var (
	detector            Detector
	isEKSCacheSingleton IsEKSCache
	once                sync.Once
)

var (
	getInClusterConfig  = func() (*rest.Config, error) { return rest.InClusterConfig() }
	getKubernetesClient = func(confs *rest.Config) (kubernetes.Interface, error) { return kubernetes.NewForConfig(confs) }
	// NewDetector creates a new singleton detector for EKS
	NewDetector = func() (Detector, error) {
		var errors error
		if clientset, err := getClient(); err != nil {
			errors = err
		} else {
			detector = &EksDetector{Clientset: clientset}
		}

		return detector, errors
	}

	// IsEKS checks if the agent is running on EKS. This is done by using the kubernetes API to determine if the aws-auth
	// configmap exists in the kube-system namespace
	IsEKS = func() IsEKSCache {
		once.Do(func() {
			var errors error
			var value bool
			// Create eks detector
			eksDetector, err := NewDetector()
			if err != nil {
				errors = err
			}

			if eksDetector != nil {
				// Make HTTP GET request
				awsAuth, err := eksDetector.getConfigMap(authConfigNamespace, authConfigConfigMap)
				if err == nil {
					value = awsAuth != nil
				}
			}
			isEKSCacheSingleton = IsEKSCache{Value: value, Err: errors}
		})

		return isEKSCacheSingleton
	}
)

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
