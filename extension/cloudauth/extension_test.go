// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudauth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestExtension_Retrieve(t *testing.T) {
	ext := &Extension{
		logger: zaptest.NewLogger(t),
		config: &Config{},
	}

	// No credentials yet.
	_, err := ext.Retrieve(context.Background())
	assert.Error(t, err)

	// Set credentials.
	expiry := time.Now().Add(time.Hour)
	ext.credentials = &ststypes.Credentials{
		AccessKeyId:     aws.String("AKID"),
		SecretAccessKey: aws.String("SECRET"),
		SessionToken:    aws.String("TOKEN"),
		Expiration:      &expiry,
	}

	creds, err := ext.Retrieve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "AKID", creds.AccessKeyID)
	assert.Equal(t, "SECRET", creds.SecretAccessKey)
	assert.Equal(t, "TOKEN", creds.SessionToken)
	assert.Equal(t, "cloudauth/oidc", creds.Source)
	assert.True(t, creds.CanExpire)
	assert.WithinDuration(t, expiry, creds.Expires, time.Second)
}

func TestExtension_NextRefreshInterval(t *testing.T) {
	ext := &Extension{
		logger: zaptest.NewLogger(t),
		config: &Config{},
	}

	// No credentials — returns minRefreshInterval.
	assert.Equal(t, minRefreshInterval, ext.nextRefreshInterval())

	// Credentials expiring in 30 minutes.
	// Interval = 30min - 5min buffer - hostJitter (0 to 5min) = 20-25min.
	expiry := time.Now().Add(30 * time.Minute)
	ext.credentials = &ststypes.Credentials{Expiration: &expiry}
	interval := ext.nextRefreshInterval()
	assert.True(t, interval >= 19*time.Minute && interval <= 26*time.Minute,
		"expected interval between 19-26min, got %v", interval)

	// Credentials expiring in 2 minutes (less than buffer).
	expiry = time.Now().Add(2 * time.Minute)
	ext.credentials = &ststypes.Credentials{Expiration: &expiry}
	assert.Equal(t, minRefreshInterval, ext.nextRefreshInterval())
}

func TestHostJitter(t *testing.T) {
	max := 5 * time.Minute
	j := hostJitter(max)
	assert.True(t, j >= 0 && j < max, "jitter %v out of range [0, %v)", j, max)
	// Deterministic: same host always returns the same value.
	assert.Equal(t, j, hostJitter(max))
}

func TestExtension_Shutdown_CleansUp(t *testing.T) {
	ext := &Extension{
		logger: zaptest.NewLogger(t),
		config: &Config{},
		done:   make(chan struct{}),
	}

	instMu.Lock()
	instance = ext
	instMu.Unlock()

	require.NoError(t, ext.Shutdown(context.Background()))
	assert.Nil(t, GetExtension())
}

func TestExtension_IsActive(t *testing.T) {
	ext := &Extension{
		logger: zaptest.NewLogger(t),
		config: &Config{},
	}
	assert.False(t, ext.IsActive())

	expiry := time.Now().Add(time.Hour)
	ext.credentials = &ststypes.Credentials{Expiration: &expiry}
	assert.True(t, ext.IsActive())
}

func TestAzureProvider_Name(t *testing.T) {
	p := NewAzureProvider()
	assert.Equal(t, "azure", p.Name())
}

func TestAzureProvider_GetToken_MockIMDS(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Metadata") != "true" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"access_token":"mock-azure-token","expires_in":"3600"}`)
	}))
	defer server.Close()

	p := &AzureProvider{client: server.Client()}
	assert.Equal(t, "azure", p.Name())
}
