// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package subprocess // import "github.com/aws/amazon-cloudwatch-agent/receiver/jmxreceiver/internal/subprocess"

import (
	"os/exec"
)

func applyOSSpecificCmdModifications(_ *exec.Cmd) {}
