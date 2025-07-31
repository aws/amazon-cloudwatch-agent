// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nvme

import (
	"errors"
	"fmt"
	"math"
)

// InstanceStoreMetrics represents the parsed metrics from the Instance Store NVMe log page 0xC0.
// This structure matches the binary layout described in the design document (bytes 0-95).
type InstanceStoreMetrics struct {
	Magic                 uint32 // 0xEC2C0D7E validation
	Reserved              uint32 // Skip
	ReadOps               uint64 // Cumulative counter
	WriteOps              uint64 // Cumulative counter
	ReadBytes             uint64 // Cumulative counter
	WriteBytes            uint64 // Cumulative counter
	TotalReadTime         uint64 // Cumulative (nanoseconds)
	TotalWriteTime        uint64 // Cumulative (nanoseconds)
	EBSIOPSExceeded       uint64 // Skip - not applicable to Instance Store
	EBSThroughputExceeded uint64 // Skip - not applicable to Instance Store
	EC2IOPSExceeded       uint64 // Cumulative counter
	EC2ThroughputExceeded uint64 // Cumulative counter
	QueueLength           uint64 // Point-in-time gauge
	// Histogram data (bytes 96+) is skipped in initial implementation
}

const (
	// InstanceStoreMagicNumber is the magic number that identifies Instance Store devices
	InstanceStoreMagicNumber = 0xEC2C0D7E
)

var (
	ErrInvalidInstanceStoreMagic = errors.New("invalid Instance Store magic number")
	ErrParseInstanceStoreLogPage = errors.New("failed to parse Instance Store log page")
	ErrDeviceAccess              = errors.New("device access failed")
	ErrIoctlFailed               = errors.New("ioctl operation failed")
	ErrInsufficientPermissions   = errors.New("insufficient permissions for device access")
	ErrDeviceNotFound            = errors.New("device not found")
	ErrBufferOverflow            = errors.New("buffer overflow detected")
)

// safeUint64ToInt64 converts a uint64 value to int64 with overflow detection.
// Returns an error if the value exceeds the maximum int64 value.
func safeUint64ToInt64(value uint64) (int64, error) {
	if value > math.MaxInt64 {
		return 0, fmt.Errorf("value %d is too large for int64", value)
	}
	return int64(value), nil
}
