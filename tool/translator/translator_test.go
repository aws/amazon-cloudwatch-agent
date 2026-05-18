// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package translator

import (
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
