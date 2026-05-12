// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package eksdetector

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"k8s.io/client-go/rest"
)

type IsEKSCache struct {
	Value bool
	Err   error
}

var (
	getInClusterConfig = func() (*rest.Config, error) { return rest.InClusterConfig() }
	getEnv             = os.Getenv
	// IsEKS is a function variable that can be overridden in tests
	IsEKS = isEKS

	// Cache for the EKS detection result
	isEKSCacheSingleton IsEKSCache
	once                sync.Once
)

// isEKS checks if the agent is running on EKS by extracting the "iss" field from the service account token and
// checking if it contains "eks". The result is cached to avoid repeated expensive operations.
func isEKS() IsEKSCache {
	once.Do(func() {
		// Fast path: check IRSA and Pod Identity env vars (no I/O)
		if checkEnvVars() {
			isEKSCacheSingleton = IsEKSCache{Value: true, Err: nil}
			return
		}

		// Fallback: parse JWT token issuer
		var err error
		var value bool
		issuer, err := getIssuer()
		if len(issuer) > 0 && err == nil {
			value = strings.Contains(strings.ToLower(issuer), "eks")
		}
		isEKSCacheSingleton = IsEKSCache{Value: value, Err: err}
	})
	return isEKSCacheSingleton
}

// checkEnvVars checks IRSA and Pod Identity environment variables for EKS indicators.
// Returns true if either env var indicates an EKS environment.
func checkEnvVars() bool {
	if strings.Contains(getEnv("AWS_WEB_IDENTITY_TOKEN_FILE"), "eks.amazonaws.com") {
		return true
	}
	if strings.Contains(getEnv("AWS_CONTAINER_AUTHORIZATION_TOKEN_FILE"), "eks-pod-identity") {
		return true
	}
	return false
}

// getIssuer retrieves the issuer ("iss") from the service account token
func getIssuer() (string, error) {
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
