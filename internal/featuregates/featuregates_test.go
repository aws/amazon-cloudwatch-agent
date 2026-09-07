// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package featuregates

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
)

func TestCanEnable(t *testing.T) {
	reg := featuregate.NewRegistry()
	assert.False(t, CanEnable(reg, ProfilesSupportGateID))

	_, err := reg.Register(ProfilesSupportGateID, featuregate.StageAlpha)
	require.NoError(t, err)
	assert.True(t, CanEnable(reg, ProfilesSupportGateID))
}

func TestCanEnable_Deprecated(t *testing.T) {
	reg := featuregate.NewRegistry()
	_, err := reg.Register(ProfilesSupportGateID, featuregate.StageDeprecated,
		featuregate.WithRegisterToVersion("v0.1.0"))
	require.NoError(t, err)
	assert.False(t, CanEnable(reg, ProfilesSupportGateID))
}

func TestEnable(t *testing.T) {
	reg := featuregate.NewRegistry()
	gate, err := reg.Register(ProfilesSupportGateID, featuregate.StageAlpha)
	require.NoError(t, err)
	assert.False(t, gate.IsEnabled())

	require.NoError(t, Enable(reg, ProfilesSupportGateID))
	assert.True(t, gate.IsEnabled())
}

func TestEnable_MissingGateIsNoop(t *testing.T) {
	reg := featuregate.NewRegistry()
	require.NoError(t, Enable(reg, ProfilesSupportGateID))
}
