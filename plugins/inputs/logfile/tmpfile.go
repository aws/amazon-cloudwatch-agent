// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build !windows
// +build !windows

package logfile

import (
	"os"
)

func createTempFile(dir, prefix string) (*os.File, error) {
	return os.CreateTemp(dir, prefix)
}
