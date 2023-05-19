// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetJsonSchema(t *testing.T) {
	jsonFile, err := os.ReadFile("./schema.json")
	if err != nil {
		require.NoError(t, err)
	}
	assert.Equal(t, string(jsonFile), GetJsonSchema(), "Json schema is inconsistent")
}

func TestGetFormattedPath(t *testing.T) {
	assert.Equal(t, "/metrics/metrics_collected/cpu/resources/1", GetFormattedPath("(root).metrics.metrics_collected.cpu.resources.1"))
	assert.Equal(t, "/metrics/metrics_collected/cpu", GetFormattedPath("(root).metrics.metrics_collected.cpu"))
}
