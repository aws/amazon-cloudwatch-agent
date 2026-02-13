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
	"go.uber.org/zap/zaptest"
)

func TestExtension_IsActive(t *testing.T) {
	ext := &Extension{logger: zaptest.NewLogger(t), config: &Config{}}

	// Not active before token fetch
	assert.False(t, ext.IsActive())

	// Active with future expiry
	ext.mu.Lock()
	ext.lastExpiry = time.Now().Add(time.Hour)
	ext.mu.Unlock()
	assert.True(t, ext.IsActive())

	// Not active after expiry
	ext.mu.Lock()
	ext.lastExpiry = time.Now().Add(-time.Hour)
	ext.mu.Unlock()
	assert.False(t, ext.IsActive())
}

func TestExtension_RefreshToken(t *testing.T) {
	tmpDir := t.TempDir()
	tokenFile := filepath.Join(tmpDir, "test-token")

	t.Run("success", func(t *testing.T) {
		ext := &Extension{
			logger:    zaptest.NewLogger(t),
			config:    &Config{RoleARN: "arn:aws:iam::123456789012:role/test"},
			provider:  &mockTokenProvider{token: "test-token", expiry: time.Hour},
			tokenFile: tokenFile,
		}

		err := ext.refreshToken(context.Background())
		require.NoError(t, err)

		// Verify token written with correct permissions
		content, err := os.ReadFile(tokenFile)
		require.NoError(t, err)
		assert.Equal(t, "test-token", string(content))

		info, err := os.Stat(tokenFile)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

		// Verify expiry tracked
		ext.mu.RLock()
		assert.False(t, ext.lastExpiry.IsZero())
		ext.mu.RUnlock()
	})

	t.Run("provider error", func(t *testing.T) {
		ext := &Extension{
			logger:    zaptest.NewLogger(t),
			config:    &Config{RoleARN: "arn:aws:iam::123456789012:role/test"},
			provider:  &mockTokenProvider{err: assert.AnError},
			tokenFile: tokenFile,
		}

		err := ext.refreshToken(context.Background())
		assert.Error(t, err)
	})
}

func TestExtension_Shutdown(t *testing.T) {
	tmpDir := t.TempDir()
	tokenFile := filepath.Join(tmpDir, "test-token")

	// Create token file
	require.NoError(t, os.WriteFile(tokenFile, []byte("test"), 0600))

	ext := &Extension{
		logger:    zaptest.NewLogger(t),
		config:    &Config{},
		tokenFile: tokenFile,
		done:      make(chan struct{}),
	}

	instMu.Lock()
	instance = ext
	instMu.Unlock()

	require.NoError(t, ext.Shutdown(context.Background()))

	// Verify cleanup
	_, err := os.Stat(tokenFile)
	assert.True(t, os.IsNotExist(err))
	assert.Nil(t, GetExtension())
}

type mockTokenProvider struct {
	token  string
	expiry time.Duration
	err    error
}

func (m *mockTokenProvider) Name() string                         { return "mock" }
func (m *mockTokenProvider) IsAvailable(ctx context.Context) bool { return m.err == nil }
func (m *mockTokenProvider) GetToken(ctx context.Context) (string, time.Duration, error) {
	if m.err != nil {
		return "", 0, m.err
	}
	return m.token, m.expiry, nil
}
