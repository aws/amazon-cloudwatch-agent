// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package fdlimit

const monitoredFiles = 5

func GetHardLimitForAllowedMonitorFiles() int {
	return CurrentFileDescriptorLimit() - monitoredFiles
}