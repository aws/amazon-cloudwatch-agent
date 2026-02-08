// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package eksdetector

import (
	"encoding/base64"
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
