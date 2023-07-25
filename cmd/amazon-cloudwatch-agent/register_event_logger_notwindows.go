// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build !windows
// +build !windows

package main

// RegisterEventLogger is for supporting Windows Event, it should only be created when running on Windows
// To minimize duplicate code for Windows vs non-Windows build amazon-cloudwatch-agent.go main class,
// create this dummy method so amazon-cloudwatch-agent.go can be build independent to the OS. Because of this method is
// invoked inside the if statement of "if runtime.GOOS == "windows" && windowsRunAsService() {" effectively it
// is unreachable, but Go compiler needs to see this exits to build.
func RegisterEventLogger() error {
	// Unreachable code, do nothing.
	return nil
}
