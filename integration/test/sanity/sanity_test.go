// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package sanity

import (
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/integration/test/utils"
)

func TestAgentStatus(t *testing.T) {
	utils.RunShellScript("resources/verifyLinuxCtlScript.sh")
}
