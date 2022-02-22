// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// +build linux
// +build integration

package sanity

import (
	"github.com/aws/amazon-cloudwatch-agent/integration/test"
	"testing"
)

func TestAgentStatus(t *testing.T) {
	test.RunShellScript("resources/verifyLinuxCtlScript.sh")
}
