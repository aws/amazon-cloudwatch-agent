// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build (linux || windows) && integration
// +build linux windows
// +build integration

package sanity

import (
	"testing"
	"runtime"
	"github.com/aws/amazon-cloudwatch-agent/integration/test"
)

func TestAgentStatus(t *testing.T) {
	os := runtime.GOOS
	
	switch os {
		case "windows":
			test.RunPowerShellScript("resources/verifyWindowsCtlScript.ps1")
		default: 
			test.RunShellScript("resources/verifyLinuxCtlScript.sh")
	}
	
}
