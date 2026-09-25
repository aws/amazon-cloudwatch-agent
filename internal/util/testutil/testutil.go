// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func GetJson(t *testing.T, path string) map[string]interface{} {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(content, &result))
	return result
}

func GetConf(t *testing.T, path string) *confmap.Conf {
	t.Helper()
	conf, err := confmaptest.LoadConf(path)
	require.NoError(t, err)
	return conf
}

func GetConfWithOverrides(t *testing.T, path string, overrides map[string]any) *confmap.Conf {
	t.Helper()
	conf, err := confmaptest.LoadConf(path)
	require.NoError(t, err)
	err = conf.Merge(confmap.NewFromStringMap(overrides))
	require.NoError(t, err)
	return conf
}

// IsolateAWSSharedConfigEnv makes SDK region and profile lookups deterministic: the shared
// credentials and config files are pointed at nonexistent paths under a temp dir, and every
// env var that supplies a region or profile is cleared. Returns the temp dir. Use this rather
// than HOME, which translator/util.CheckAndSetHomeDir overwrites.
func IsolateAWSSharedConfigEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(dir, "no-such-config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(dir, "no-such-credentials"))
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	for _, k := range []string{"AWS_PROFILE", "AWS_DEFAULT_PROFILE", "AWS_REGION", "AWS_DEFAULT_REGION"} {
		t.Setenv(k, "")
		require.NoError(t, os.Unsetenv(k))
	}
	return dir
}
