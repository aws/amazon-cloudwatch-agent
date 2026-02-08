//go:build !linux && !windows

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nvidia

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStubChecker_AlwaysReturnsFalse(t *testing.T) {
	c := newChecker()

	// Stub implementation should always return false
	assert.False(t, c.hasNvidiaDevice())
	assert.False(t, c.hasDriverFiles())
}
