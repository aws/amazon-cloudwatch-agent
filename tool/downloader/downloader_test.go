// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package downloader

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
)

func TestRunDownloader_DefaultOtel(t *testing.T) {
	outputDir := t.TempDir()
	err := RunDownloader("ec2", "default:otel", outputDir, "", "default", false)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outputDir, "default_otel.tmp"))
	require.NoError(t, err)

	expected, ok := config.DefaultJSONConfigFor("otel", false, false)
	require.True(t, ok)
	assert.JSONEq(t, expected, string(content))
}

func TestRunDownloader_DefaultBare(t *testing.T) {
	outputDir := t.TempDir()
	err := RunDownloader("ec2", "default", outputDir, "", "default", false)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outputDir, "default.tmp"))
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(content, &parsed))
	assert.Contains(t, parsed, "metrics")
}

func TestRunDownloader_DefaultUnknown(t *testing.T) {
	outputDir := t.TempDir()
	err := RunDownloader("ec2", "default:invalid", outputDir, "", "default", false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `unknown default config "invalid"`)
}

func TestRunDownloader_DefaultOtelRemove(t *testing.T) {
	outputDir := t.TempDir()
	// Pre-create the config file that remove will delete
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "default_otel"), []byte("{}"), 0600))

	err := RunDownloader("ec2", "default:otel", outputDir, "", "remove", false)
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(outputDir, "default_otel"))
	assert.True(t, os.IsNotExist(err))
}

func TestRunDownloader_DefaultEmptyName(t *testing.T) {
	outputDir := t.TempDir()
	err := RunDownloader("ec2", "default:", outputDir, "", "default", false)
	assert.EqualError(t, err, `unknown default config ""`)
}

func TestRunDownloader_DefaultExtraColons(t *testing.T) {
	outputDir := t.TempDir()
	err := RunDownloader("ec2", "default:otel:extra", outputDir, "", "default", false)
	assert.EqualError(t, err, `unknown default config "otel:extra"`)
}

// TestRunDownloaderFromFlags_LogsToStdout asserts the downloader routes the Go log package to stdout
// before doing anything else; startup stderr aborts Windows user-data/SSM installs. Flags are
// intentionally invalid so the call fails fast before any network access.
func TestRunDownloaderFromFlags_LogsToStdout(t *testing.T) {
	origWriter := log.Writer()
	t.Cleanup(func() { log.SetOutput(origWriter) })
	log.SetOutput(os.Stderr)

	empty := ""
	flags := map[string]*string{
		"mode":            &empty,
		"download-source": &empty,
		"output-dir":      &empty,
		"config":          &empty,
		"multi-config":    &empty,
		"dualstack":       &empty,
	}
	err := RunDownloaderFromFlags(flags)
	require.Error(t, err, "empty flags should fail validation")
	require.Same(t, os.Stdout, log.Writer(), "downloader must route log output to stdout")
}
