// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && (386 || arm) && !go1.16
// +build linux
// +build 386 arm
// +build !go1.16

package cmdutil

import (
	"golang.org/x/sys/unix"
)

// go1.15 and before: use unix raw syscall. Can be removed once minimum go version has been bumped.

func setUid(uid int) (err error) {
	_, _, e1 := unix.RawSyscall(unix.SYS_SETUID32, uintptr(uid), 0, 0)
	if e1 != 0 {
		err = e1
	}
	return
}

func setGid(gid int) (err error) {
	_, _, e1 := unix.RawSyscall(unix.SYS_SETGID32, uintptr(gid), 0, 0)
	if e1 != 0 {
		err = e1
	}
	return
}
