// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux || darwin || freebsd || netbsd || openbsd
// +build linux darwin freebsd netbsd openbsd

package fdlimit

import (
	"syscall"
)

func CurrentOpenFileLimit() (int, error) {
	var limit syscall.Rlimit

	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		return 0, err
	}

	return int(limit.Cur), nil
}
