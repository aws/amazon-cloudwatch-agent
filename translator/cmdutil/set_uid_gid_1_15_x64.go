// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && (arm64 || amd64 || mips || mipsle || mips64 || mips64le || ppc || ppc64 || ppc64le || riscv64 || s390x) && !go1.16
// +build linux
// +build arm64 amd64 mips mipsle mips64 mips64le ppc ppc64 ppc64le riscv64 s390x
// +build !go1.16

package cmdutil

import (
	"golang.org/x/sys/unix"
)

// go1.15 and before: use unix raw syscall. Can be removed once minimum go version has been bumped.

func setUid(uid int) (err error) {
	_, _, e1 := unix.RawSyscall(unix.SYS_SETUID, uintptr(uid), 0, 0)
	if e1 != 0 {
		err = e1
	}
	return
}

func setGid(gid int) (err error) {
	_, _, e1 := unix.RawSyscall(unix.SYS_SETGID, uintptr(gid), 0, 0)
	if e1 != 0 {
		err = e1
	}
	return
}
