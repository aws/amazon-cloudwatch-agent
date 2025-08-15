// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// The following code is based on https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/pkg/metrics/nvme.go

// Copyright 2024 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the 'License');
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an 'AS IS' BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build linux

package nvme

import (
	"bytes"
	"encoding/binary"
	"errors"
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

var (
	ErrInvalidEBSMagic           = errors.New("invalid EBS magic number")
	ErrInvalidInstanceStoreMagic = errors.New("invalid Instance Store magic number")
	ErrParseLogPage              = errors.New("failed to parse log page")
	ErrUnsupportedMagic          = errors.New("unsupported magic number")
)

// GetMetrics retrieves NVMe metrics by reading the log page from the NVMe device at the given path.
func GetMetrics(devicePath string) (any, error) {
	data, err := getNVMEMetrics(devicePath)
	if err != nil {
		return nil, err
	}

	return parseLogPage(data)
}

// getNVMEMetrics retrieves NVMe metrics by reading the log page from the NVMe device at the given path.
func getNVMEMetrics(devicePath string) ([]byte, error) {
	f, err := os.OpenFile(devicePath, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("getNVMEMetrics: error opening device: %w", err)
	}
	defer f.Close()

	data, err := nvmeReadLogPage(f.Fd(), logID)
	if err != nil {
		return nil, fmt.Errorf("getNVMEMetrics: error reading log page %w", err)
	}

	return data, nil
}

// nvmeReadLogPage reads an NVMe log page via an ioctl system call.
func nvmeReadLogPage(fd uintptr, logID uint8) ([]byte, error) {
	data := make([]byte, 4096) // 4096 bytes is the length of the log page.
	bufferLen := len(data)

	if bufferLen > math.MaxUint32 {
		return nil, errors.New("nvmeReadLogPage: bufferLen exceeds MaxUint32")
	}

	cmd := nvmePassthruCommand{
		opcode:  0x02,
		addr:    uint64(uintptr(unsafe.Pointer(&data[0]))),
		nsid:    1,
		dataLen: uint32(bufferLen),
		cdw10:   uint32(logID) | (1023 << 16),
	}

	status, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, nvmeIoctlAdminCmd, uintptr(unsafe.Pointer(&cmd)))
	if errno != 0 {
		return nil, fmt.Errorf("nvmeReadLogPage: ioctl error %w", errno)
	}
	if status != 0 {
		return nil, fmt.Errorf("nvmeReadLogPage: ioctl command failed with status %d", status)
	}
	return data, nil
}

// parseLogPage parses the binary data from an EBS or Instance Store log page into the corresponding struct.
func parseLogPage(data []byte) (any, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("%w: input too short", ErrParseLogPage)
	}

	magic64 := binary.LittleEndian.Uint64(data[0:8])
	magic32 := binary.LittleEndian.Uint32(data[0:4])

	switch {
	case magic64 == ebsMagic:
		var metrics EBSMetrics
		reader := bytes.NewReader(data)
		if err := binary.Read(reader, binary.LittleEndian, &metrics); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrParseLogPage, err)
		}
		if metrics.EBSMagic != ebsMagic {
			return nil, ErrInvalidEBSMagic
		}
		return metrics, nil

	case magic32 == instanceStoreMagic:
		var metrics InstanceStoreMetrics
		reader := bytes.NewReader(data)
		if err := binary.Read(reader, binary.LittleEndian, &metrics); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrParseLogPage, err)
		}
		if metrics.Magic != instanceStoreMagic {
			return nil, ErrInvalidInstanceStoreMagic
		}
		return metrics, nil

	default:
		return nil, ErrUnsupportedMagic
	}
}
