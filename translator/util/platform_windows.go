// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package util

import (
	"syscall"
)

var windowsVersion int

// Windows version list https://msdn.microsoft.com/en-us/library/windows/desktop/ms724832(v=vs.85).aspx
func GetOSMajorVersion() (int, error) {
	return windowsVersion, nil
}

func init() {
	// To determine Windows version
	modkernel32 := syscall.NewLazyDLL("kernel32.dll")
	procGetVersion := modkernel32.NewProc("GetVersion")
	v, _, _ := procGetVersion.Call()
	windowsVersion = int(byte(v))
}
