// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultJSONConfigFor_Otel(t *testing.T) {
	cfg, ok := DefaultJSONConfigFor("otel")
	require.True(t, ok)
	assert.JSONEq(t, defaultOtelConfig, cfg)
}

func TestDefaultJSONConfigFor_Unknown(t *testing.T) {
	_, ok := DefaultJSONConfigFor("unknown")
	assert.False(t, ok)
}

func TestDefaultJSONConfigFor_Empty(t *testing.T) {
	_, ok := DefaultJSONConfigFor("")
	assert.False(t, ok)
}
