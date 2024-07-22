// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package volume

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	ebsSerialPrefix    = "vol"
	ebsSerialSeparator = "-"

	sysBlockPath = "/sys/block/"
	serialFile   = "device/serial"

	loopDevicePrefix = "loop"
)

type hostProvider struct {
	osReadDir  func(string) ([]os.DirEntry, error)
	osReadFile func(string) ([]byte, error)
}

func newHostProvider() Provider {
	return &hostProvider{
		osReadDir:  os.ReadDir,
		osReadFile: os.ReadFile,
	}
}

func (p *hostProvider) DeviceToSerialMap() (map[string]string, error) {
	result := map[string]string{}
	dirs, err := p.osReadDir(sysBlockPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read %s: %w", sysBlockPath, err)
	}
	for _, dir := range dirs {
		deviceName := dir.Name()
		// skip loop devices
		if strings.HasPrefix(deviceName, loopDevicePrefix) {
			continue
		}
		serial, _ := p.osReadFile(serialFilePath(deviceName))
		serial = bytes.TrimSpace(serial)
		if len(serial) > 0 {
			result[deviceName] = formatSerial(string(serial))
		}
	}
	if len(result) == 0 {
		return nil, errors.New("no devices/serials found")
	}
	return result, nil
}

func formatSerial(serial string) string {
	suffix, ok := strings.CutPrefix(serial, ebsSerialPrefix)
	if !ok || strings.HasPrefix(suffix, ebsSerialSeparator) {
		return serial
	}
	return ebsSerialPrefix + ebsSerialSeparator + suffix
}

func serialFilePath(deviceName string) string {
	return filepath.Join(sysBlockPath, deviceName, serialFile)
}
