// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package envconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsUsageDataEnabled(t *testing.T) {
	assert.True(t, getUsageDataEnabled())

	t.Setenv(CWAGENT_USAGE_DATA, "TRUE")
	assert.True(t, getUsageDataEnabled())

	t.Setenv(CWAGENT_USAGE_DATA, "INVALID")
	assert.True(t, getUsageDataEnabled())

	t.Setenv(CWAGENT_USAGE_DATA, "FALSE")
	assert.False(t, getUsageDataEnabled())
}

func TestIsRunningInContainer(t *testing.T) {
	assert.False(t, IsRunningInContainer())

	t.Setenv(RunInContainer, "TRUE")
	assert.False(t, IsRunningInContainer())

	t.Setenv(RunInContainer, TrueValue)
	assert.True(t, IsRunningInContainer())
}
