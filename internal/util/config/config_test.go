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
	dir := t.TempDir()
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
	assert.Len(t, got, 14)
	assert.Equal(t, []string{
		"-otelconfig", filepath.Join(dir, "1.yaml"),
		"-otelconfig", filepath.Join(dir, "11.yaml"),
		"-otelconfig", filepath.Join(dir, "2.yaml"),
		"-otelconfig", filepath.Join(dir, "bar.yaml"),
		"-otelconfig", filepath.Join(dir, "baz.yaml"),
		"-otelconfig", filepath.Join(dir, "foo.yaml"),
		"-otelconfig", paths.YamlConfigPath,
	}, got)
}
