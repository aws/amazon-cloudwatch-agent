// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultJSONConfigFor_Otel(t *testing.T) {
	cfg, ok := DefaultJSONConfigFor("otel")
	require.True(t, ok)

	expected, err := os.ReadFile("defaults/otel.json")
	require.NoError(t, err)
	assert.JSONEq(t, string(expected), cfg)
}

func TestDefaultJSONConfigFor_Unknown(t *testing.T) {
	_, ok := DefaultJSONConfigFor("unknown")
	assert.False(t, ok)
}

func TestDefaultJSONConfigFor_Empty(t *testing.T) {
	_, ok := DefaultJSONConfigFor("")
	assert.False(t, ok)
}
