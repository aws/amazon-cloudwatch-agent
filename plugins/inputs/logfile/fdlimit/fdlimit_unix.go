// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux || darwin || freebsd || netbsd || openbsd
// +build linux darwin freebsd netbsd openbsd

package fdlimit

import (
	"log"
	"syscall"
)

func currentFileDescriptorLimit() int {
	var limit syscall.Rlimit

	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		log.Printf("E! Failed to get file descriptor limit: %v \n", err)
		return 0
	}

	return int(limit.Cur)
}