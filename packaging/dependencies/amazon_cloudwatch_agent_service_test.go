// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package dependencies_test

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAmazonCloudWatchAgentServiceKillMode(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)

	data, err := os.ReadFile(filepath.Join(filepath.Dir(file), "amazon-cloudwatch-agent.service"))
	require.NoError(t, err)

	assert.True(t, regexp.MustCompile(`(?m)^KillMode=mixed$`).Match(data),
		"service must use KillMode=mixed so session dbus children are reaped on stop (#2195)")
	assert.False(t, regexp.MustCompile(`(?m)^KillMode=process$`).Match(data),
		"KillMode=process leaves dbus-daemon --session in the unit cgroup across restarts")
}
