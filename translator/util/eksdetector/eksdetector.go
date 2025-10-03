// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package eksdetector

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/client-go/rest"
)

type IsEKSCache struct {
	Value bool
	Err   error
}

var (
	getInClusterConfig = func() (*rest.Config, error) { return rest.InClusterConfig() }
	IsEKS = isEKS
	NewDetector = isEKS 
)

// isEKS checks if the agent is running on EKS by extracting the "iss" field from the service account token and
// checking if it contains "eks"
func isEKS() IsEKSCache {
	issuer, err := getIssuer()
	if err != nil {
		return IsEKSCache{Value: false, Err: err}
	}

	value := strings.Contains(strings.ToLower(issuer), "eks")
	return IsEKSCache{Value: value, Err: nil}
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
