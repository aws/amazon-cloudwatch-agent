// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux || freebsd || netbsd || openbsd
// +build linux freebsd netbsd openbsd

package sanity

import (
	"github.com/aws/amazon-cloudwatch-agent/integration/test"
)

func SanityCheck() {
	test.RunShellScript("resources/verifyLinuxCtlScript.sh")
}
