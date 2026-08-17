// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package translator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/translator"
	translatorcontext "github.com/aws/amazon-cloudwatch-agent/translator/context"
	translatorutil "github.com/aws/amazon-cloudwatch-agent/translator/util"
)

func TestTranslate_OnlyYAML(t *testing.T) {
	orig := translatorutil.DetectRegion
	translatorutil.DetectRegion = func(string, map[string]string) (string, string) {
		return "us-east-1", "mock"
	}
	defer func() { translatorutil.DetectRegion = orig }()

	translator.ResetMessages()
	translatorcontext.ResetContext()

	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "config.yaml"), nil, 0600))

	tomlPath := filepath.Join(tmpDir, "output.toml")
	// Pre-create the YAML output file to verify it gets removed.
	yamlPath := filepath.Join(tmpDir, yamlConfigFileName)
	require.NoError(t, os.WriteFile(yamlPath, nil, 0600))

	ct, err := NewConfigTranslator("linux", "", tmpDir, tomlPath, "ec2", "", "default")
	require.NoError(t, err)

	assert.NoError(t, ct.Translate())
	assert.FileExists(t, tomlPath)
	assert.NoFileExists(t, yamlPath)
}

func TestTranslate_RetainsCustomKeysAndClearsStaleTranslatorManagedKeys(t *testing.T) {
	orig := translatorutil.DetectRegion
	translatorutil.DetectRegion = func(string, map[string]string) (string, string) {
		return "us-east-1", "mock"
	}
	defer func() { translatorutil.DetectRegion = orig }()

	translator.ResetMessages()
	translatorcontext.ResetContext()

	// Ensure no ambient environment value drives the managed key.
	t.Setenv("CWAGENT_LOGS_BACKPRESSURE_MODE", "")

	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "config.json.tmp"), []byte(`{}`), 0600))
	// Pre-populate env-config.json with a stale managed key and a custom key.
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "env-config.json"),
		[]byte(`{"CWAGENT_LOGS_BACKPRESSURE_MODE":"fd_release","MY_CUSTOM_VAR":"keep"}`),
		0600,
	))

	tomlPath := filepath.Join(tmpDir, "output.toml")
	ct, err := NewConfigTranslator("linux", "", tmpDir, tomlPath, "ec2", "", "default")
	require.NoError(t, err)
	require.NoError(t, ct.Translate())

	result := map[string]string{}
	data, err := os.ReadFile(filepath.Join(tmpDir, "env-config.json"))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &result))

	// Custom keys survive translation.
	assert.Equal(t, "keep", result["MY_CUSTOM_VAR"])
	// Managed keys no longer configured are cleared rather than re-emitted from the old file.
	_, exists := result["CWAGENT_LOGS_BACKPRESSURE_MODE"]
	assert.False(t, exists)
}
