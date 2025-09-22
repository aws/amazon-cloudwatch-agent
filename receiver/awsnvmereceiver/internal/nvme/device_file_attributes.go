// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nvme

import (
	"errors"
	"fmt"
)

type DeviceFileAttributes struct {
	controller int
	namespace  int
	partition  int
	deviceName string
}

func ParseNvmeDeviceFileName(device string) (DeviceFileAttributes, error) {
	controller := -1
	namespace := -1
	partition := -1

	_, _ = fmt.Sscanf(device, "nvme%dn%dp%d", &controller, &namespace, &partition)

	if controller == -1 {
		return DeviceFileAttributes{deviceName: device}, errors.New("unable to parse device name")
	}

	return DeviceFileAttributes{
		controller: controller,
		namespace:  namespace,
		partition:  partition,
		deviceName: device,
	}, nil
}

func (n *DeviceFileAttributes) Controller() int {
	return n.controller
}

func (n *DeviceFileAttributes) Namespace() int {
	return n.namespace
}

func (n *DeviceFileAttributes) Partition() int {
	return n.partition
}

func (n *DeviceFileAttributes) BaseDeviceName() (string, error) {
	if n.Controller() == -1 {
		return "", errors.New("unable to re-create device name due to missing controller id")
	}

	return fmt.Sprintf("nvme%d", n.Controller()), nil
}

func (n *DeviceFileAttributes) DeviceName() string {
	return n.deviceName
}
