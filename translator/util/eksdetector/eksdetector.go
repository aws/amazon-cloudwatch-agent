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
)

// IsEKS checks if the agent is running on EKS by extracting the "iss" field from the service account token and
// checking if it contains "eks"
func IsEKS() IsEKSCache {
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
		return "", fmt.Errorf("EKS detection failed: no service account token available (not running in Kubernetes): %w", err)
	}

	token := conf.BearerToken
	if token == "" {
		return "", fmt.Errorf("EKS detection failed: no service account token available (not running in Kubernetes)")
	}

	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("EKS detection failed: invalid service account token format (expected JWT with header.payload.signature)")
	}

	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("EKS detection failed: invalid service account token format (failed to decode payload): %w", err)
	}

	var claims map[string]any
	if err = json.Unmarshal(decoded, &claims); err != nil {
		return "", fmt.Errorf("EKS detection failed: invalid service account token format (malformed JSON payload): %w", err)
	}

	iss, ok := claims["iss"].(string)
	if !ok {
		return "", fmt.Errorf("EKS detection failed: service account token missing issuer claim (expected 'iss' field in token)")
	}

	return iss, nil
}
