// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package volume

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/mitchellh/mapstructure"
)

const (
	lsblkPath = "/usr/bin/lsblk"

	ebsSerialPrefix    = "vol"
	ebsSerialSeparator = "-"
)

var (
	lsblkArgs = []string{"--json", "--output", "name,serial"}
)

type ListBlockDeviceOutput struct {
	BlockDevices []BlockDevice `json:"blockdevices"`
}

type BlockDevice struct {
	Name     string        `json:"name"`
	Serial   string        `json:"serial"`
	Children []BlockDevice `json:"children"`
}

func getBlockDevices() (map[string]string, error) {
	stdout, err := exec.Command(lsblkPath, lsblkArgs...).Output()
	if err != nil {
		return nil, fmt.Errorf("unable to run lsblk: %w", err)
	}
	var raw any
	if err = json.Unmarshal(stdout, &raw); err != nil {
		return nil, fmt.Errorf("unable to unmarshal lsblk JSON output: %w", err)
	}
	var output ListBlockDeviceOutput
	if err = mapstructure.WeakDecode(raw, &output); err != nil {
		return nil, fmt.Errorf("unable to unmarshal into structured output: %w", err)
	}
	result := map[string]string{}
	for _, device := range output.BlockDevices {
		addDevice(result, device)
	}
	if len(result) == 0 {
		return nil, errors.New("no devices/serials found")
	}
	return result, nil
}

func addDevice(result map[string]string, device BlockDevice) {
	for _, child := range device.Children {
		addDevice(result, child)
	}
	if device.Name != "" && device.Serial != "" {
		result[device.Name] = formatSerial(device.Serial)
	}
}

func formatSerial(serial string) string {
	suffix, ok := strings.CutPrefix(serial, ebsSerialPrefix)
	if !ok {
		return serial
	}
	return ebsSerialPrefix + ebsSerialSeparator + suffix
}
