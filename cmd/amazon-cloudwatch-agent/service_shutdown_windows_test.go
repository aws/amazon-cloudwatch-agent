// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExitAfterServiceRunExitsZero(t *testing.T) {
	orig := osExit
	t.Cleanup(func() { osExit = orig })

	var got int
	var called bool
	osExit = func(code int) {
		got = code
		called = true
	}

	exitAfterServiceRun()

	assert.True(t, called, "expected osExit to be called")
	assert.Equal(t, 0, got)
}
