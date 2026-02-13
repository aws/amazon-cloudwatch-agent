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

func TestExtension_RefreshToken(t *testing.T) {
	tmpDir := t.TempDir()
	tokenFile := filepath.Join(tmpDir, "test-token")

	t.Run("success", func(t *testing.T) {
		ext := &Extension{
			logger:        zaptest.NewLogger(t),
			config:        &Config{},
			tokenProvider: &mockTokenProvider{token: "test-token", expiry: time.Hour},
			tokenFile:     tokenFile,
		}

		expiry, err := ext.refreshToken(context.Background())
		require.NoError(t, err)
		assert.True(t, expiry.After(time.Now()))

		content, err := os.ReadFile(tokenFile)
		require.NoError(t, err)
		assert.Equal(t, "test-token", string(content))
	})

	t.Run("provider error", func(t *testing.T) {
		ext := &Extension{
			logger:        zaptest.NewLogger(t),
			config:        &Config{},
			tokenProvider: &mockTokenProvider{err: assert.AnError},
			tokenFile:     tokenFile,
		}
		_, err := ext.refreshToken(context.Background())
		assert.Error(t, err)
	})
}

func TestExtension_Shutdown(t *testing.T) {
	tmpDir := t.TempDir()
	tokenFile := filepath.Join(tmpDir, "test-token")
	require.NoError(t, os.WriteFile(tokenFile, []byte("test"), 0600))

	ext := &Extension{
		logger:    zaptest.NewLogger(t),
		config:    &Config{},
		tokenFile: tokenFile,
		done:      make(chan struct{}),
	}
	require.NoError(t, ext.Shutdown(context.Background()))

	_, err := os.Stat(tokenFile)
	assert.True(t, os.IsNotExist(err))
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
