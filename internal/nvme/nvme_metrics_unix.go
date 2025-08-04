//go:build linux

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nvme

import (
	"fmt"
	"math"
	"os"
	"syscall"
	"unsafe"
)

// As defined in <linux/nvme_ioctl.h>.
type nvmePassthruCommand struct {
	opcode      uint8
	flags       uint8
	rsvd1       uint16
	nsid        uint32
	cdw2        uint32
	cdw3        uint32
	metadata    uint64
	addr        uint64
	metadataLen uint32
	dataLen     uint32
	cdw10       uint32
	cdw11       uint32
	cdw12       uint32
	cdw13       uint32
	cdw14       uint32
	cdw15       uint32
	timeoutMs   uint32
	result      uint32
}

// GetInstanceStoreMetrics retrieves Instance Store metrics from the specified device path.
// It reads log page 0xC0 and parses the Instance Store specific metrics.
func GetInstanceStoreMetrics(devicePath string) (InstanceStoreMetrics, error) {
	data, err := getInstanceStoreNVMEMetrics(devicePath)
	if err != nil {
		return InstanceStoreMetrics{}, fmt.Errorf("failed to retrieve Instance Store metrics from device %s: %w", devicePath, err)
	}

	metrics, err := ParseInstanceStoreLogPage(data)
	if err != nil {
		return InstanceStoreMetrics{}, fmt.Errorf("failed to parse Instance Store log page for device %s: %w", devicePath, err)
	}

	return metrics, nil
}

// getInstanceStoreNVMEMetrics retrieves Instance Store NVMe metrics by reading log page 0xC0 from the NVMe device.
func getInstanceStoreNVMEMetrics(devicePath string) ([]byte, error) {
	f, err := os.OpenFile(devicePath, os.O_RDWR, 0)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: device %s not found", ErrDeviceNotFound, devicePath)
		}
		if os.IsPermission(err) {
			return nil, fmt.Errorf("%w: insufficient permissions to access device %s (CAP_SYS_ADMIN required)", ErrInsufficientPermissions, devicePath)
		}
		return nil, fmt.Errorf("%w: failed to open device %s: %w", ErrDeviceAccess, devicePath, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			// Log close error but don't override the main error
			// This would require a logger, but we don't have one at this level
			// The caller should handle logging
		}
	}()

	data, err := nvmeReadInstanceStoreLogPage(f.Fd(), 0xC0)
	if err != nil {
		return nil, fmt.Errorf("failed to read log page 0xC0 from device %s: %w", devicePath, err)
	}

	return data, nil
}

// nvmeReadInstanceStoreLogPage reads Instance Store NVMe log page 0xC0 via an ioctl system call.
func nvmeReadInstanceStoreLogPage(fd uintptr, logID uint8) ([]byte, error) {
	data := make([]byte, 4096) // 4096 bytes is the length of the log page
	bufferLen := len(data)

	if bufferLen > math.MaxUint32 {
		return nil, fmt.Errorf("%w: buffer length %d exceeds MaxUint32", ErrBufferOverflow, bufferLen)
	}

	// Validate buffer bounds to prevent potential security issues
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: zero-length buffer provided", ErrBufferOverflow)
	}

	cmd := nvmePassthruCommand{
		opcode:  0x02, // NVMe Get Log Page command
		addr:    uint64(uintptr(unsafe.Pointer(&data[0]))),
		nsid:    1,
		dataLen: uint32(bufferLen),
		cdw10:   uint32(logID) | (1024 << 16), // Log page ID and number of dwords
	}

	status, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, 0xC0484E41, uintptr(unsafe.Pointer(&cmd)))
	if errno != 0 {
		// Use enhanced ioctl error handling for better error classification and recovery
		return nil, EnhanceIoctlError(errno, fmt.Sprintf("read Instance Store log page 0x%X", logID), fmt.Sprintf("fd:%d", fd))
	}
	if status != 0 {
		// NVMe command status codes - provide more meaningful error messages
		switch status {
		case 0x02:
			return nil, fmt.Errorf("%w: invalid log page ID 0x%X", ErrIoctlFailed, logID)
		case 0x0A:
			return nil, fmt.Errorf("%w: log page 0x%X not supported by device", ErrIoctlFailed, logID)
		case 0x16:
			return nil, fmt.Errorf("%w: insufficient privileges for log page access", ErrInsufficientPermissions)
		default:
			return nil, fmt.Errorf("%w: NVMe command failed with status 0x%X for log page 0x%X", ErrIoctlFailed, status, logID)
		}
	}
	return data, nil
}

// GetEBSMetrics retrieves EBS metrics from the specified device path.
// It reads log page 0xD0 and parses the EBS specific metrics.
func GetEBSMetrics(devicePath string) (EBSMetrics, error) {
	data, err := getEBSNVMEMetrics(devicePath)
	if err != nil {
		return EBSMetrics{}, fmt.Errorf("failed to retrieve EBS metrics from device %s: %w", devicePath, err)
	}

	metrics, err := ParseEBSLogPage(data)
	if err != nil {
		return EBSMetrics{}, fmt.Errorf("failed to parse EBS log page for device %s: %w", devicePath, err)
	}

	return metrics, nil
}

// getEBSNVMEMetrics retrieves EBS NVMe metrics by reading log page 0xD0 from the NVMe device.
func getEBSNVMEMetrics(devicePath string) ([]byte, error) {
	f, err := os.OpenFile(devicePath, os.O_RDWR, 0)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: device %s not found", ErrDeviceNotFound, devicePath)
		}
		if os.IsPermission(err) {
			return nil, fmt.Errorf("%w: insufficient permissions to access device %s (CAP_SYS_ADMIN required)", ErrInsufficientPermissions, devicePath)
		}
		return nil, fmt.Errorf("%w: failed to open device %s: %w", ErrDeviceAccess, devicePath, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			// Log close error but don't override the main error
			// This would require a logger, but we don't have one at this level
			// The caller should handle logging
		}
	}()

	data, err := nvmeReadEBSLogPage(f.Fd(), 0xD0)
	if err != nil {
		return nil, fmt.Errorf("failed to read log page 0xD0 from device %s: %w", devicePath, err)
	}

	return data, nil
}

// nvmeReadEBSLogPage reads EBS NVMe log page 0xD0 via an ioctl system call.
func nvmeReadEBSLogPage(fd uintptr, logID uint8) ([]byte, error) {
	data := make([]byte, 4096) // 4096 bytes is the length of the log page
	bufferLen := len(data)

	if bufferLen > math.MaxUint32 {
		return nil, fmt.Errorf("%w: buffer length %d exceeds MaxUint32", ErrBufferOverflow, bufferLen)
	}

	// Validate buffer bounds to prevent potential security issues
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: zero-length buffer provided", ErrBufferOverflow)
	}

	cmd := nvmePassthruCommand{
		opcode:  0x02, // NVMe Get Log Page command
		addr:    uint64(uintptr(unsafe.Pointer(&data[0]))),
		nsid:    1,
		dataLen: uint32(bufferLen),
		cdw10:   uint32(logID) | (1024 << 16), // Log page ID and number of dwords
	}

	status, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, 0xC0484E41, uintptr(unsafe.Pointer(&cmd)))
	if errno != 0 {
		// Use enhanced ioctl error handling for better error classification and recovery
		return nil, EnhanceIoctlError(errno, fmt.Sprintf("read EBS log page 0x%X", logID), fmt.Sprintf("fd:%d", fd))
	}
	if status != 0 {
		// NVMe command status codes - provide more meaningful error messages
		switch status {
		case 0x02:
			return nil, fmt.Errorf("%w: invalid log page ID 0x%X", ErrIoctlFailed, logID)
		case 0x0A:
			return nil, fmt.Errorf("%w: log page 0x%X not supported by device", ErrIoctlFailed, logID)
		case 0x16:
			return nil, fmt.Errorf("%w: insufficient privileges for log page access", ErrInsufficientPermissions)
		default:
			return nil, fmt.Errorf("%w: NVMe command failed with status 0x%X for log page 0x%X", ErrIoctlFailed, status, logID)
		}
	}
	return data, nil
}
