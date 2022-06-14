// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package fdlimit

// On Windows, Go uses the CreateFile API, which is limited to 16K files; therefore, non-changeable from within a running process
// MySQL has encountered the same issue https://bugs.mysql.com/bug.php?id=24509
// An example of how go-ethereum handle file descriptors https://github.com/ethereum/go-ethereum/blob/8a134014b4b370b4a3632e32a2fc8e84ee2b6947/common/fdlimit/fdlimit_windows.go
const hardLimitFileDescriptor = 16384

func CurrentOpenFileLimit() (int, error) {
	return hardLimitFileDescriptor, nil
}
