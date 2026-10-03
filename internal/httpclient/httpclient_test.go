// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package httpclient

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequest_ReturnsOnFirstSuccess(t *testing.T) {
	callCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success-body"))
	}))
	defer server.Close()

	client := New()
	body, err := client.Request(server.URL)

	require.NoError(t, err)
	assert.Equal(t, "success-body", string(body))
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount), "should only call once on success")
}

func TestRequest_RetriesOnFailureThenSucceeds(t *testing.T) {
	callCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&callCount, 1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success-after-retry"))
	}))
	defer server.Close()

	client := New()
	body, err := client.Request(server.URL)

	require.NoError(t, err)
	assert.Equal(t, "success-after-retry", string(body))
	assert.Equal(t, int32(2), atomic.LoadInt32(&callCount), "should retry once then succeed")
}

func TestRequest_ReturnsErrorAfterMaxRetries(t *testing.T) {
	callCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := New()
	body, err := client.Request(server.URL)

	require.Error(t, err)
	assert.Nil(t, body)
	assert.Equal(t, int32(3), atomic.LoadInt32(&callCount), "should try maxRetries times")
	assert.Contains(t, err.Error(), "status code: 500")
}
