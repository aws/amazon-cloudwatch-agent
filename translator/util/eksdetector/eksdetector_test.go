// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package eksdetector

import (
	"encoding/base64"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
)

// Tests EKS resource detector running in EKS environment
func TestEKS(t *testing.T) {
	resetCacheForTesting() // Reset cache for consistency

	payload := `{"iss":"https://oidc.eks.us-west-2.amazonaws.com/id/someid"}`
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	dummyToken := "header." + encodedPayload + ".signature"

	getInClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{BearerToken: dummyToken}, nil
	}

	isEks := IsEKS()
	assert.True(t, isEks.Value)
	assert.NoError(t, isEks.Err)
}

func TestNonEKS(t *testing.T) {
	resetCacheForTesting()

	payload := `{"iss":"https://kubernetes.default.svc.cluster.local"}`
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	dummyToken := "header." + encodedPayload + ".signature"

	getInClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{BearerToken: dummyToken}, nil
	}

	isEks := IsEKS()
	assert.False(t, isEks.Value)
	assert.NoError(t, isEks.Err)
}

func TestEmptyToken(t *testing.T) {
	resetCacheForTesting()

	getInClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{BearerToken: ""}, nil
	}

	isEks := IsEKS()
	assert.False(t, isEks.Value)
	assert.Error(t, isEks.Err)
	assert.Contains(t, isEks.Err.Error(), "empty token in config")
}

func Test_getIssuer(t *testing.T) {
	payload := `{"iss":"https://oidc.eks.us-west-2.amazonaws.com/id/someid"}`
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	dummyToken := "header." + encodedPayload + ".signature"

	getInClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{BearerToken: dummyToken}, nil
	}

	issuer, err := getIssuer()
	assert.NoError(t, err)
	assert.Equal(t, "https://oidc.eks.us-west-2.amazonaws.com/id/someid", issuer)
}

// resetCacheForTesting resets the EKS detection cache - only used in tests
func resetCacheForTesting() {
	isEKSCacheSingleton = IsEKSCache{}
	once = sync.Once{}
}

func TestEKS_IRSA_EnvVar(t *testing.T) {
	resetCacheForTesting()
	origGetEnv := getEnv
	defer func() { getEnv = origGetEnv }()

	getEnv = func(key string) string {
		if key == "AWS_WEB_IDENTITY_TOKEN_FILE" {
			return "/var/run/secrets/eks.amazonaws.com/serviceaccount/token"
		}
		return ""
	}
	getInClusterConfig = func() (*rest.Config, error) {
		return nil, fmt.Errorf("should not be called")
	}

	result := IsEKS()
	assert.True(t, result.Value)
	assert.NoError(t, result.Err)
}

func TestEKS_PodIdentity_EnvVar(t *testing.T) {
	resetCacheForTesting()
	origGetEnv := getEnv
	defer func() { getEnv = origGetEnv }()

	getEnv = func(key string) string {
		if key == "AWS_CONTAINER_AUTHORIZATION_TOKEN_FILE" {
			return "/var/run/secrets/pods.eks-pod-identity.amazonaws.com/serviceaccount/token"
		}
		return ""
	}
	getInClusterConfig = func() (*rest.Config, error) {
		return nil, fmt.Errorf("should not be called")
	}

	result := IsEKS()
	assert.True(t, result.Value)
	assert.NoError(t, result.Err)
}

func TestEKS_EnvVarsAbsent_FallsThrough(t *testing.T) {
	resetCacheForTesting()
	origGetEnv := getEnv
	defer func() { getEnv = origGetEnv }()

	getEnv = func(_ string) string { return "" }

	payload := `{"iss":"https://oidc.eks.us-west-2.amazonaws.com/id/someid"}`
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	dummyToken := "header." + encodedPayload + ".signature"
	getInClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{BearerToken: dummyToken}, nil
	}

	result := IsEKS()
	assert.True(t, result.Value)
	assert.NoError(t, result.Err)
}

func TestNonEKS_EnvVarsAbsent_NonEKSToken(t *testing.T) {
	resetCacheForTesting()
	origGetEnv := getEnv
	defer func() { getEnv = origGetEnv }()

	getEnv = func(_ string) string { return "" }

	payload := `{"iss":"https://kubernetes.default.svc.cluster.local"}`
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	dummyToken := "header." + encodedPayload + ".signature"
	getInClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{BearerToken: dummyToken}, nil
	}

	result := IsEKS()
	assert.False(t, result.Value)
	assert.NoError(t, result.Err)
}

func TestEKS_PartialEnvVars_IRSAWithoutEKS(t *testing.T) {
	resetCacheForTesting()
	origGetEnv := getEnv
	defer func() { getEnv = origGetEnv }()

	getEnv = func(key string) string {
		if key == "AWS_WEB_IDENTITY_TOKEN_FILE" {
			return "/var/run/secrets/some-other-provider/token"
		}
		return ""
	}

	payload := `{"iss":"https://oidc.eks.us-west-2.amazonaws.com/id/someid"}`
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	dummyToken := "header." + encodedPayload + ".signature"
	getInClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{BearerToken: dummyToken}, nil
	}

	result := IsEKS()
	assert.True(t, result.Value)
	assert.NoError(t, result.Err)
}

func TestEKS_BothEnvVarsSet(t *testing.T) {
	resetCacheForTesting()
	origGetEnv := getEnv
	defer func() { getEnv = origGetEnv }()

	getEnv = func(key string) string {
		switch key {
		case "AWS_WEB_IDENTITY_TOKEN_FILE":
			return "/var/run/secrets/eks.amazonaws.com/serviceaccount/token"
		case "AWS_CONTAINER_AUTHORIZATION_TOKEN_FILE":
			return "/var/run/secrets/pods.eks-pod-identity.amazonaws.com/serviceaccount/token"
		}
		return ""
	}
	getInClusterConfig = func() (*rest.Config, error) {
		return nil, fmt.Errorf("should not be called")
	}

	result := IsEKS()
	assert.True(t, result.Value)
	assert.NoError(t, result.Err)
}
