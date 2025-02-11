// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package eksdetector

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Detector interface {
	getIssuer() (string, error)
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

	// IsEKS checks if the agent is running on EKS by extracting the "iss"
	// field from the service account token and checking if it contains "eks".
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
				issuer, err := eksDetector.getIssuer()
				fmt.Println("issuer: ", issuer)
				if err == nil {
					value = strings.Contains(strings.ToLower(issuer), "eks")
				}
			}
			isEKSCacheSingleton = IsEKSCache{Value: value, Err: errors}
		})

		return isEKSCacheSingleton
	}
)

// getIssuer retrieves the issuer ("iss") from the service account token.
func (d *EksDetector) getIssuer() (string, error) {
	conf, err := getInClusterConfig()
	if err != nil {
		return "", fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	token := conf.BearerToken
	if token == "" {
		return "", fmt.Errorf("empty token in config")
	}

	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("missing payload")
	}

	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("failed to decode token payload: %w", err)
	}

	var claims map[string]interface{}
	if err = json.Unmarshal(decoded, &claims); err != nil {
		return "", fmt.Errorf("failed to unmarshal token payload: %w", err)
	}

	iss, ok := claims["iss"].(string)
	if !ok {
		return "", fmt.Errorf("issuer field not found in token")
	}

	return iss, nil
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
