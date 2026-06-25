// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package envconfig

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "env-config.json")
	content := `{"KEY1": "val1", "KEY2": "val2"}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))

	result, err := ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"KEY1": "val1", "KEY2": "val2"}, result)
}

func TestReadFile_EmptyPath(t *testing.T) {
	result, err := ReadFile("")
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestReadFile_MissingFile(t *testing.T) {
	_, err := ReadFile(filepath.Join(t.TempDir(), "nonexistent.json"))
	assert.True(t, errors.Is(err, fs.ErrNotExist))
}

func TestReadFile_CorruptReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "env-config.json")
	require.NoError(t, os.WriteFile(path, []byte("not valid json"), 0600))

	_, err := ReadFile(path)
	assert.Error(t, err)
}

func TestLoadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "env-config.json")
	content := `{"MY_TEST_VAR": "hello", "MY_OTHER_VAR": "world"}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))

	t.Setenv("MY_TEST_VAR", "")
	t.Setenv("MY_OTHER_VAR", "")

	require.NoError(t, LoadFile(path))

	assert.Equal(t, "hello", os.Getenv("MY_TEST_VAR"))
	assert.Equal(t, "world", os.Getenv("MY_OTHER_VAR"))
}

func TestLoadFile_MissingFile(t *testing.T) {
	err := LoadFile(filepath.Join(t.TempDir(), "nonexistent.json"))
	assert.True(t, errors.Is(err, fs.ErrNotExist))
}

func TestLoadFile_EmptyPath(t *testing.T) {
	assert.NoError(t, LoadFile(""))
}

func TestMergeFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "env-config.json")
	existing := map[string]string{"KEEP_ME": "original", "OVERWRITE_ME": "old"}
	data, err := json.MarshalIndent(existing, "", "\t")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0600))

	require.NoError(t, MergeFile(path, map[string]string{
		"OVERWRITE_ME": "new",
		"NEW_KEY":      "added",
	}))

	result, err := ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "original", result["KEEP_ME"])
	assert.Equal(t, "new", result["OVERWRITE_ME"])
	assert.Equal(t, "added", result["NEW_KEY"])
}

func TestMergeFile_CreatesIfMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "env-config.json")

	require.NoError(t, MergeFile(path, map[string]string{"KEY": "value"}))

	result, err := ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"KEY": "value"}, result)
}

func TestMergeFile_RemovesStaleKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "env-config.json")
	existing := map[string]string{"STALE_KEY": "old", "KEEP_ME": "retained"}
	data, err := json.MarshalIndent(existing, "", "\t")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0600))

	err = MergeFile(path, map[string]string{"NEW_KEY": "new"}, "STALE_KEY")
	require.NoError(t, err)

	result, err := ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "retained", result["KEEP_ME"])
	assert.Equal(t, "new", result["NEW_KEY"])
	_, hasStale := result["STALE_KEY"]
	assert.False(t, hasStale)
}

func TestMergeFile_CorruptReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "env-config.json")
	require.NoError(t, os.WriteFile(path, []byte("{ broken json"), 0600))

	err := MergeFile(path, map[string]string{"NEW_KEY": "new"}, "STALE_KEY")
	assert.Error(t, err)

	// File is left as-is so the original content is recoverable.
	content, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, "{ broken json", string(content))
}

func TestMergeFile_IOErrorReturnsError(t *testing.T) {
	// A directory can't be read as a file, producing a non-ErrNotExist error.
	dir := t.TempDir()
	err := MergeFile(dir, map[string]string{"KEY": "value"})
	assert.Error(t, err)
}

func TestMergeFile_EmptyPath(t *testing.T) {
	assert.NoError(t, MergeFile("", map[string]string{"KEY": "value"}))
}
