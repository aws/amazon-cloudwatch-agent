// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package eksdetector

import (
	"fmt"
	"strings"
	"sync"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Detector interface {
	getServerVersion() (string, error)
}

type EksDetector struct {
	Clientset kubernetes.Interface
}

type IsEKSCache struct {
	Value bool
	Err   error
}

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

	// IsEKS checks if the agent is running on EKS. This is done by using the kubernetes API
	// to determine if the server version string contains "eks".
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
				// Check server version
				serverVersion, err := eksDetector.getServerVersion()
				if err == nil {
					value = strings.Contains(strings.ToLower(serverVersion), "eks")
				}
			}
			isEKSCacheSingleton = IsEKSCache{Value: value, Err: errors}
		})

		return isEKSCacheSingleton
	}
)

// getServerVersion retrieves the cluster's server version
func (d *EksDetector) getServerVersion() (string, error) {
	version, err := d.Clientset.Discovery().ServerVersion()
	if err != nil {
		return "", fmt.Errorf("failed to retrieve server version: %w", err)
	}

	return version.GitVersion, nil
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
