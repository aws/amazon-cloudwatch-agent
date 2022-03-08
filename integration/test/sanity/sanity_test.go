// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package sanity

import (
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/integration/test"
)

func TestAgentStatus(t *testing.T) {
	test.RunShellScript("resources/verifyLinuxCtlScript.sh")
}
