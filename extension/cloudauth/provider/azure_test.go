// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAzureProvider_GetToken_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "true", r.Header.Get("Metadata"))
		assert.Contains(t, r.URL.RawQuery, "api-version="+defaultAzureIMDSAPIVersion)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"access_token":"mock-jwt-token","expires_in":"3600"}`)
	}))
	defer server.Close()

	p := &AzureProvider{client: server.Client(), endpoint: server.URL}
	token, expiry, err := p.GetToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "mock-jwt-token", token)
	assert.Equal(t, 3600, int(expiry.Seconds()))
}

func TestAzureProvider_GetToken_EmptyToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"access_token":"","expires_in":"3600"}`)
	}))
	defer server.Close()

	p := &AzureProvider{client: server.Client(), endpoint: server.URL}
	_, _, err := p.GetToken(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty access_token")
}

func TestAzureProvider_GetToken_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"error":"identity not found"}`)
	}))
	defer server.Close()

	p := &AzureProvider{client: server.Client(), endpoint: server.URL}
	_, _, err := p.GetToken(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

func TestAzureProvider_GetToken_DefaultExpiry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"access_token":"token","expires_in":"invalid"}`)
	}))
	defer server.Close()

	p := &AzureProvider{client: server.Client(), endpoint: server.URL}
	token, expiry, err := p.GetToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "token", token)
	assert.Equal(t, 3600, int(expiry.Seconds()))
}

func TestAzureProvider_IsAvailable_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Metadata") != "True" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		assert.Contains(t, r.URL.RawQuery, "api-version="+defaultAzureProbeAPIVersion)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"location":"eastus"}`)
	}))
	defer server.Close()

	p := &AzureProvider{client: server.Client(), endpoint: server.URL, probeEndpoint: server.URL}
	assert.True(t, p.IsAvailable(context.Background()))
}

func TestAzureProvider_IsAvailable_NotAzure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	p := &AzureProvider{client: server.Client(), endpoint: server.URL, probeEndpoint: server.URL}
	assert.False(t, p.IsAvailable(context.Background()))
}
