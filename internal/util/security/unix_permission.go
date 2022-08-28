// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// go:build !windows
// +build !windows

package security

import (
	"fmt"
	"syscall"
)

// CheckFileRights check that the given filename has been protected by the owner.
// If the owner is changed, they need at least the sudo permission to override the owner.
func CheckFileRights(filePath string) error {
	var stat syscall.Stat_t
	if err := syscall.Stat(filePath, &stat); err != nil {
		return fmt.Errorf("Cannot get file's stat %s: %v", filePath, err)
	}

	// Check the owner of binary has read, write, exec. 
	if !(stat.Mode&(syscall.S_IXUSR) == 0 || stat.Mode&(syscall.S_IRUSR) == 0 || stat.Mode&(syscall.S_IWUSR) == 0) {
		return nil
	}
	
	// Check the owner of file has read, write
	if !(stat.Mode&(syscall.S_IRUSR) == 0 || stat.Mode&(syscall.S_IWUSR) == 0) {
		return nil
	}
	
	return fmt.Errorf("File's owner does not have enough permission at path %s", filePath)
}
