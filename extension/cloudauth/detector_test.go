// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudauth

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockProvider struct {
	name      string
	available bool
}

func (m *mockProvider) Name() string                       { return m.name }
func (m *mockProvider) IsAvailable(_ context.Context) bool { return m.available }
func (m *mockProvider) GetToken(_ context.Context) (string, time.Duration, error) {
	return "", 0, nil
}

func TestDetectProvider_TokenFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(f, []byte("my-jwt-token"), 0600))

	p, err := DetectProvider(context.Background(), f)
	require.NoError(t, err)
	assert.Equal(t, "file", p.Name())
}

func TestDetectProvider_TokenFileMissing(t *testing.T) {
	_, err := DetectProvider(context.Background(), "/nonexistent/token")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestDetectProvider_AutoDetect(t *testing.T) {
	orig := registeredProviders
	defer func() { registeredProviders = orig }()

	registeredProviders = []func() TokenProvider{
		func() TokenProvider { return &mockProvider{name: "first", available: false} },
		func() TokenProvider { return &mockProvider{name: "second", available: true} },
	}

	p, err := DetectProvider(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, "second", p.Name())
}

func TestDetectProvider_NoneAvailable(t *testing.T) {
	orig := registeredProviders
	defer func() { registeredProviders = orig }()

	registeredProviders = []func() TokenProvider{
		func() TokenProvider { return &mockProvider{name: "only", available: false} },
	}

	_, err := DetectProvider(context.Background(), "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no OIDC provider detected")
}
