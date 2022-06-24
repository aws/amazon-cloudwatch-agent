// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package sanity

import (
	"github.com/aws/amazon-cloudwatch-agent/integration/test"
)

func SanityCheck() {
	test.RunPowerShellScript("resources/verifyWindowsCtlScript.ps1")
}