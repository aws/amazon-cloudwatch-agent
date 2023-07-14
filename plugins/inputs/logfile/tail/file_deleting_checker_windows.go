// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package tail

import (
	"syscall"
)

func (tail *Tail) isFileDeleted() bool {
	if tail.file == nil {
		return false
	}
	var d syscall.ByHandleFileInformation
	err := syscall.GetFileInformationByHandle(syscall.Handle(tail.file.Fd()), &d)
	if err != nil {
		tail.Logger.Errorf("Got a error when calling GetFileInformationByHandle: +%v", err)
		return false
	}

	return d.NumberOfLinks == 0
}
