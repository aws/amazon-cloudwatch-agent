// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package tail

import (
	"os"

	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/logfile/tail/winfile"
)

func OpenFile(name string) (file *os.File, err error) {
	return winfile.OpenFile(name, os.O_RDONLY, 0)
}
