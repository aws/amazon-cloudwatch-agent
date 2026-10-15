// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
)

func TestGetOTELConfigArgs(t *testing.T) {
	orig := paths.YamlConfigPath
	t.Cleanup(func() { paths.YamlConfigPath = orig })

	t.Run("WithoutDefaultYAML", func(t *testing.T) {
		paths.YamlConfigPath = filepath.Join(t.TempDir(), paths.YAML)
		got := GetOTELConfigArgs("/not/valid/path")
		assert.Empty(t, got)
	})

	t.Run("WithDefaultYAML", func(t *testing.T) {
		yamlPath := filepath.Join(t.TempDir(), paths.YAML)
		require.NoError(t, os.WriteFile(yamlPath, nil, 0600))
		paths.YamlConfigPath = yamlPath
		got := GetOTELConfigArgs("/not/valid/path")
		assert.Equal(t, []string{"-otelconfig", yamlPath}, got)
	})

	t.Run("WithDirYAMLs", func(t *testing.T) {
		yamlPath := filepath.Join(t.TempDir(), paths.YAML)
		require.NoError(t, os.WriteFile(yamlPath, nil, 0600))
		paths.YamlConfigPath = yamlPath

		dir := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(dir, "bunchofyaml"), 0644))
		for _, name := range []string{
			"foo.yaml",
			"bar.yaml",
			"not-yaml",    // skipped
			"ignore.json", // skipped
			"baz.yaml",
			"1.yaml",
			"2.yaml",
			"11.yaml",
		} {
			f, err := os.Create(filepath.Join(dir, name))
			require.NoError(t, err)
			require.NoError(t, f.Close())
		}
		got := GetOTELConfigArgs(dir)
		assert.Equal(t, []string{
			"-otelconfig", filepath.Join(dir, "1.yaml"),
			"-otelconfig", filepath.Join(dir, "11.yaml"),
			"-otelconfig", filepath.Join(dir, "2.yaml"),
			"-otelconfig", filepath.Join(dir, "bar.yaml"),
			"-otelconfig", filepath.Join(dir, "baz.yaml"),
			"-otelconfig", filepath.Join(dir, "foo.yaml"),
			"-otelconfig", yamlPath,
		}, got)
	})
}
