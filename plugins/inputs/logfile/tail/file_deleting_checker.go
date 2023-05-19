// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux || darwin
// +build linux darwin

package tail

import (
	"syscall"
)

func (tail *Tail) isFileDeleted() bool {
	if tail.file == nil {
		return false
	}
	fileInfo, err := tail.file.Stat()
	if err != nil || fileInfo == nil {
		return false
	}
	sysInfo := fileInfo.Sys()
	if sysInfo == nil {
		return false
	}
	stat, ok := sysInfo.(*syscall.Stat_t)
	if !ok || stat == nil {
		return false
	}

	return stat.Nlink == 0

}
