// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package eksdetector

import (
	"context"
	"fmt"
	"os"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
)

type Detector interface {
	getConfigMap(namespace string, name string) (map[string]string, error)
	getWorkloadType() string
}

type EksDetector struct {
	Clientset kubernetes.Interface
}

type IsEKSCache struct {
	Value    bool
	Workload string
	Err      error
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
			var workloadType string
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

				// Get workload type
				workloadType = eksDetector.getWorkloadType()
			}
			isEKSCacheSingleton = IsEKSCache{Value: value, Workload: workloadType, Err: errors}
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

func (d *EksDetector) getWorkloadType() string {
	podName := os.Getenv("POD_NAME")
	namespace := os.Getenv("K8S_NAMESPACE")

	if podName == "" || namespace == "" {
		fmt.Println(fmt.Errorf("POD_NAME/K8S_NAMESPACE environment variables not set").Error())
		return ""
	}

	pod, err := d.Clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		fmt.Println(fmt.Errorf("failed to get pod for workload type detection: %v", err).Error())
		return ""
	}

	for _, owner := range pod.OwnerReferences {
		switch owner.Kind {
		case "DaemonSet":
			return config.DaemonSet
		case "StatefulSet":
			return config.StatefulSet
		case "ReplicaSet":
			return config.Deployment
		}
	}

	return ""
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
