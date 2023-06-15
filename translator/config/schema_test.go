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
	require.NoError(t, err)
	assert.Equal(t, string(jsonFile), GetJsonSchema(), "Json schema is inconsistent")
}

func TestOverwriteSchema(t *testing.T) {
	originalSchema := GetJsonSchema()
	newSchema := "new schema"
	OverwriteSchema(newSchema)
	assert.NotEqual(t, originalSchema, GetJsonSchema())
	assert.Equal(t, newSchema, GetJsonSchema())
}

func TestGetFormattedPath(t *testing.T) {
	assert.Equal(t, "/metrics/metrics_collected/cpu/resources/1", GetFormattedPath("(root).metrics.metrics_collected.cpu.resources.1"))
	assert.Equal(t, "/metrics/metrics_collected/cpu", GetFormattedPath("(root).metrics.metrics_collected.cpu"))
}
