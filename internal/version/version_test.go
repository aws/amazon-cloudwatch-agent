// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package version

import (
	"fmt"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersion(t *testing.T) {
	expectedVersion := "Unknown"
	expectedFullVersion := fmt.Sprintf("CWAgent/%s (%s; %s; %s)",
		expectedVersion,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH)
	assert.Equal(t, expectedVersion, Number())
	assert.Equal(t, expectedFullVersion, Full())

	expectedVersion = "TEST_VERSION"
	expectedFullVersion = fmt.Sprintf("CWAgent/%s (%s; %s; %s)",
		expectedVersion,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH)
	filePath, err := FilePath()
	require.NoError(t, err)
	err = os.WriteFile(filePath, []byte(expectedVersion), 0644)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Remove(filePath)
	})

	actualVersion := readVersionFile()
	assert.Equal(t, expectedVersion, actualVersion)
	assert.Equal(t, expectedFullVersion, buildFullVersion(actualVersion))
}
