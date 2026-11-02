// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
)

// TestPrepareConfig_AlwaysTranslates verifies that Linux and macOS keep
// translating on every start: the stamp functions are no-ops and the
// translator is always attempted. The translator is missing here, so the
// attempt fails; a nil error would mean the translation was skipped.
func TestPrepareConfig_AlwaysTranslates(t *testing.T) {
	dir := t.TempDir()
	origTOML := paths.TomlConfigPath
	origTranslatorBinary := paths.TranslatorBinaryPath
	t.Cleanup(func() {
		paths.TomlConfigPath = origTOML
		paths.TranslatorBinaryPath = origTranslatorBinary
	})
	paths.TomlConfigPath = filepath.Join(dir, "amazon-cloudwatch-agent.toml")
	paths.TranslatorBinaryPath = filepath.Join(dir, "config-translator-does-not-exist")
	t.Setenv("RUN_IN_CONTAINER", "")
	require.NoError(t, os.WriteFile(paths.TomlConfigPath, []byte("[agent]\n"), 0o600))

	assert.False(t, translationIsCurrent())
	writeTranslationStamp(captureTranslationInputs())
	removeTranslationStamp()

	translated, err := prepareConfig()
	require.Error(t, err)
	assert.True(t, translated)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "no stamp file may be written on this platform")
	assert.Equal(t, "amazon-cloudwatch-agent.toml", entries[0].Name())
}
