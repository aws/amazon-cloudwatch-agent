// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nvidia

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewDetector(t *testing.T) {
	d := NewDetector(slog.Default())
	require.NotNil(t, d)
}
