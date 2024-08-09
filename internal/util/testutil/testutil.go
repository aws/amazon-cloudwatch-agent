// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package testutil

import (
	"encoding/json"
	"os"
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
