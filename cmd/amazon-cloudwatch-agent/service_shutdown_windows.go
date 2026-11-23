// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package main

import "os"

// osExit is a seam for tests.
var osExit = os.Exit

// exitAfterServiceRun exits the process after the service run loop ends.
func exitAfterServiceRun() {
	osExit(0)
}
