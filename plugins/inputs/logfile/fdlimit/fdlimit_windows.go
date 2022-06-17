// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package fdlimit

// On Windows, Go uses the CreateFile API, which is limited to 16K files; therefore, non-changeable from within a running process
// MySQL has encountered the same issue https://bugs.mysql.com/bug.php?id=24509
// The default number of allowed file handles for network on Windows is 16384, so aligning with that
const hardLimitFileDescriptor int = 16384

func CurrentFileDescriptorLimit() int {
	return hardLimitFileDescriptor
}
