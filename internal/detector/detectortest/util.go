// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package detectortest

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func CmdlineArgsFromFile(t *testing.T, path string) []string {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	lines := strings.Split(string(content), "\n")

	return strings.Fields(strings.TrimSpace(lines[0]))
}
