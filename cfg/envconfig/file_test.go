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

func TestReadEnvConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "env-config.json")
	content := `{"KEY1": "val1", "KEY2": "val2"}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	result, err := ReadEnvConfigFile(path)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"KEY1": "val1", "KEY2": "val2"}, result)
}

func TestReadEnvConfigFile_EmptyPath(t *testing.T) {
	result, err := ReadEnvConfigFile("")
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestReadEnvConfigFile_MissingFile(t *testing.T) {
	_, err := ReadEnvConfigFile(filepath.Join(t.TempDir(), "nonexistent.json"))
	assert.True(t, errors.Is(err, fs.ErrNotExist))
}

func TestLoadEnvConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "env-config.json")
	content := `{"MY_TEST_VAR": "hello", "MY_OTHER_VAR": "world"}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	t.Setenv("MY_TEST_VAR", "")
	t.Setenv("MY_OTHER_VAR", "")

	require.NoError(t, LoadEnvConfigFile(path))

	assert.Equal(t, "hello", os.Getenv("MY_TEST_VAR"))
	assert.Equal(t, "world", os.Getenv("MY_OTHER_VAR"))
}

func TestLoadEnvConfigFile_MissingFile(t *testing.T) {
	err := LoadEnvConfigFile(filepath.Join(t.TempDir(), "nonexistent.json"))
	assert.True(t, errors.Is(err, fs.ErrNotExist))
}

func TestLoadEnvConfigFile_EmptyPath(t *testing.T) {
	assert.NoError(t, LoadEnvConfigFile(""))
}

func TestMergeEnvConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "env-config.json")
	existing := map[string]string{"KEEP_ME": "original", "OVERWRITE_ME": "old"}
	data, err := json.MarshalIndent(existing, "", "\t")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0644))

	require.NoError(t, MergeEnvConfigFile(path, map[string]string{
		"OVERWRITE_ME": "new",
		"NEW_KEY":      "added",
	}))

	result, err := ReadEnvConfigFile(path)
	require.NoError(t, err)
	assert.Equal(t, "original", result["KEEP_ME"])
	assert.Equal(t, "new", result["OVERWRITE_ME"])
	assert.Equal(t, "added", result["NEW_KEY"])
}

func TestMergeEnvConfigFile_CreatesIfMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "env-config.json")

	require.NoError(t, MergeEnvConfigFile(path, map[string]string{"KEY": "value"}))

	result, err := ReadEnvConfigFile(path)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"KEY": "value"}, result)
}

func TestMergeEnvConfigFile_CorruptExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "env-config.json")
	require.NoError(t, os.WriteFile(path, []byte("not valid json"), 0644))

	err := MergeEnvConfigFile(path, map[string]string{"KEY": "value"})
	assert.Error(t, err)

	// File should be unchanged
	content, _ := os.ReadFile(path)
	assert.Equal(t, "not valid json", string(content))
}

func TestReplaceEnvConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "env-config.json")
	existing := map[string]string{"STALE_KEY": "old", "KEEP_ME": "retained"}
	data, err := json.MarshalIndent(existing, "", "\t")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0644))

	err = ReplaceEnvConfigFile(path, map[string]string{"NEW_KEY": "new"}, []string{"STALE_KEY"})
	require.NoError(t, err)

	result, err := ReadEnvConfigFile(path)
	require.NoError(t, err)
	assert.Equal(t, "retained", result["KEEP_ME"])
	assert.Equal(t, "new", result["NEW_KEY"])
	_, hasStale := result["STALE_KEY"]
	assert.False(t, hasStale)
}
