// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package eksdetector

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
)

// Tests EKS resource detector running in EKS environment
func TestEKS(t *testing.T) {
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

// Tests non-EKS Kubernetes environment
func TestNonEKS(t *testing.T) {
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

// Tests error when not running in cluster
func TestNotInCluster(t *testing.T) {
	getInClusterConfig = func() (*rest.Config, error) {
		return nil, errors.New("unable to load in-cluster configuration")
	}

	isEks := IsEKS()
	assert.False(t, isEks.Value)
	assert.Error(t, isEks.Err)
	assert.Contains(t, isEks.Err.Error(), "EKS detection failed: no service account token available")
}

// Tests error when token is empty
func TestEmptyToken(t *testing.T) {
	getInClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{BearerToken: ""}, nil
	}

	isEks := IsEKS()
	assert.False(t, isEks.Value)
	assert.Error(t, isEks.Err)
	assert.Contains(t, isEks.Err.Error(), "EKS detection failed: no service account token available")
}

// Tests error when token format is invalid
func TestInvalidTokenFormat(t *testing.T) {
	getInClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{BearerToken: "invalid-token"}, nil
	}

	isEks := IsEKS()
	assert.False(t, isEks.Value)
	assert.Error(t, isEks.Err)
	assert.Contains(t, isEks.Err.Error(), "EKS detection failed: invalid service account token format")
}

// Tests error when token payload is not valid base64
func TestInvalidBase64Payload(t *testing.T) {
	dummyToken := "header.invalid-base64!@#.signature"

	getInClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{BearerToken: dummyToken}, nil
	}

	isEks := IsEKS()
	assert.False(t, isEks.Value)
	assert.Error(t, isEks.Err)
	assert.Contains(t, isEks.Err.Error(), "EKS detection failed: invalid service account token format (failed to decode payload)")
}

// Tests error when token payload is not valid JSON
func TestInvalidJSONPayload(t *testing.T) {
	invalidPayload := "not-json"
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(invalidPayload))
	dummyToken := "header." + encodedPayload + ".signature"

	getInClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{BearerToken: dummyToken}, nil
	}

	isEks := IsEKS()
	assert.False(t, isEks.Value)
	assert.Error(t, isEks.Err)
	assert.Contains(t, isEks.Err.Error(), "EKS detection failed: invalid service account token format (malformed JSON payload)")
}

// Tests error when token payload is missing issuer claim
func TestMissingIssuerClaim(t *testing.T) {
	payload := `{"sub":"system:serviceaccount:default:default"}`
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	dummyToken := "header." + encodedPayload + ".signature"

	getInClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{BearerToken: dummyToken}, nil
	}

	isEks := IsEKS()
	assert.False(t, isEks.Value)
	assert.Error(t, isEks.Err)
	assert.Contains(t, isEks.Err.Error(), "EKS detection failed: service account token missing issuer claim")
}
