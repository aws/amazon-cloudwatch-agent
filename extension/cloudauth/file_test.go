// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudauth

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileProvider_GetToken(t *testing.T) {
	f := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(f, []byte("  my-jwt-token\n"), 0600))

	fp := NewFileProvider(f)
	token, expiry, err := fp.GetToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "my-jwt-token", token)
	assert.Zero(t, expiry)
}

func TestFileProvider_GetToken_Empty(t *testing.T) {
	f := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(f, []byte("  \n"), 0600))

	fp := NewFileProvider(f)
	_, _, err := fp.GetToken(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestFileProvider_GetToken_Missing(t *testing.T) {
	fp := NewFileProvider("/nonexistent/token")
	_, _, err := fp.GetToken(context.Background())
	assert.Error(t, err)
}

func TestFileProvider_IsAvailable(t *testing.T) {
	f := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(f, []byte("tok"), 0600))

	assert.True(t, NewFileProvider(f).IsAvailable(context.Background()))
	assert.False(t, NewFileProvider("/nonexistent").IsAvailable(context.Background()))
}

func TestFileProvider_Name(t *testing.T) {
	assert.Equal(t, "file", NewFileProvider("x").Name())
}
