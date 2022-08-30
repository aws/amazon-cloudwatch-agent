// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux || freebsd || netbsd || openbsd
// +build linux freebsd netbsd openbsd

package sanity

import (
	"testing"
	"github.com/aws/amazon-cloudwatch-agent/integration/test"
)

func SanityCheck(t *testing.T) {
	err := test.RunShellScript("resources/verifyLinuxCtlScript.sh")
	if err != nil {
		t.Fatalf("Running sanity check failed")
	}
}
