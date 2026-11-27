// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build !windows
// +build !windows

package main

import "testing"

func TestExitAfterServiceRunNoopOffWindows(t *testing.T) {
	exitAfterServiceRun()
}
