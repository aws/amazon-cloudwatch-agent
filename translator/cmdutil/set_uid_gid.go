// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// +build linux
// +build go1.16

package cmdutil

import "syscall"

// go1.16 and later: use Setgid/Setuid implemented in go syscall (https://golang.org/doc/go1.16#syscall).

func setUid(uid int) (err error) {
	return syscall.Setuid(uid)
}

func setGid(gid int) (err error) {
	return syscall.Setgid(gid)
}
