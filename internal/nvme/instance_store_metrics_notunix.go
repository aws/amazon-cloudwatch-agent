//go:build !linux

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nvme

import "errors"

var ErrUnsupportedPlatform = errors.New("Instance Store metrics are only supported on Linux")

// GetInstanceStoreMetrics returns an error on non-Linux platforms.
func GetInstanceStoreMetrics(devicePath string) (InstanceStoreMetrics, error) {
	return InstanceStoreMetrics{}, ErrUnsupportedPlatform
}
