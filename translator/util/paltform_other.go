// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// +build !windows

package util

import (
	"errors"
	"fmt"
	"runtime"
)

func GetOSMajorVersion() (int, error) {
	return 0, errors.New(fmt.Sprintf("Unsupported operation on %s", runtime.GOOS))
}
