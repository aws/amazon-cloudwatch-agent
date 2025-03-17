// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nvme

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type NvmeDeviceFileAttributes struct {
	controller   int
	namespace    int
	partition    int
}

type Attribute interface {
	apply(*NvmeDeviceFileAttributes) error
}

type nvmeDeviceAttributeFunc func(*NvmeDeviceFileAttributes) error

func (f nvmeDeviceAttributeFunc) apply(e *NvmeDeviceFileAttributes) error {
	return f(e)
}

func ParseNvmeDeviceFileName(device string) (NvmeDeviceFileAttributes, error) {
	if !strings.HasPrefix(device, NvmeDevicePrefix) {
		return NvmeDeviceFileAttributes{
			controller: -1,
			namespace:  -1,
			partition:  -1,
		}, errors.New("device is not prefixed with nvme")
	}

	trimmed := strings.TrimPrefix(device, NvmeDevicePrefix)

	controllerEndIdx := strings.Index(trimmed, "n")
	if controllerEndIdx == -1 {
		controllerEndIdx = len(trimmed)
		return newNvmeDeviceFileAttributes(
			withController(substring(trimmed, 0, controllerEndIdx)),
		)
	}

	namespaceEndIdx := strings.Index(trimmed, "p")
	if namespaceEndIdx == -1 {
		namespaceEndIdx = len(trimmed)
		return newNvmeDeviceFileAttributes(
			withController(substring(trimmed, 0, controllerEndIdx)),
			withNamespace(substring(trimmed, controllerEndIdx+1, namespaceEndIdx)),
		)
	}

	return newNvmeDeviceFileAttributes(
		withController(substring(trimmed, 0, controllerEndIdx)),
		withNamespace(substring(trimmed, controllerEndIdx+1, namespaceEndIdx)),
		withPartition(substring(trimmed, namespaceEndIdx+1, len(trimmed))),
	)
}

func (n *NvmeDeviceFileAttributes) Controller() int {
	return n.controller
}

func (n *NvmeDeviceFileAttributes) Namespace() int {
	return n.namespace
}

func (n *NvmeDeviceFileAttributes) Partition() int {
	return n.partition
}

func (n *NvmeDeviceFileAttributes) BaseDeviceName() (string, error) {
	if n.Controller() == -1 {
		return "", errors.New("unable to re-create device name due to missing controller id")
	}

	return fmt.Sprintf("nvme%d", n.Controller()), nil
}

func (n *NvmeDeviceFileAttributes) DeviceName() (string, error) {
	hasNamespace := n.Namespace() != -1
	hasPartition := n.Partition() != -1

	if hasNamespace && hasPartition {
		return fmt.Sprintf("nvme%dn%dp%d", n.Controller(), n.Namespace(), n.Partition()), nil
	} else if hasNamespace {
		return fmt.Sprintf("nvme%dn%d", n.Controller(), n.Namespace()), nil
	} 

	// Fall back to BaseDeviceName if only the controller ID exists
	return n.BaseDeviceName()
}

func newNvmeDeviceFileAttributes(attributes ...Attribute) (NvmeDeviceFileAttributes, error) {
	n := &NvmeDeviceFileAttributes{
		controller: -1,
		namespace:  -1,
		partition:  -1,
	}
	var anyErr error
	for _, attribute := range attributes {
		err := attribute.apply(n)
		if err != nil {
			anyErr = err
		}
	}
	// Controller should always exist and should be a non-negative number
	if n.Controller() == -1 {
		return *n, errors.New("unable to parse controller id of nvme device")
	}
	return *n, anyErr
}

func withController(controller string) Attribute {
	c, err := convertNvmeIdStringToNum(controller)
	return nvmeDeviceAttributeFunc(func(attr *NvmeDeviceFileAttributes) error {
		attr.controller = c
		return err
	})
}

func withNamespace(namespace string) Attribute {
	n, err := convertNvmeIdStringToNum(namespace)
	return nvmeDeviceAttributeFunc(func(attr *NvmeDeviceFileAttributes) error {
		attr.namespace = n
		return err
	})
}

func withPartition(partition string) Attribute {
	p, err := convertNvmeIdStringToNum(partition)
	return nvmeDeviceAttributeFunc(func(attr *NvmeDeviceFileAttributes) error {
		attr.partition = p
		return err
	})
}

// substring returns the slice of a string, or an empty string if the bounds
// are invaild
func substring(s string, l, r int) string {
	if l < 0 {
		return ""
	}
	if r > len(s) {
		return ""
	}
	if l >= r {
		return ""
	}

	return s[l:r]
}

func convertNvmeIdStringToNum(a string) (int, error) {
	if a == "" {
		return -1, errors.New("nvme device attribute is empty")
	}
	i, err := strconv.Atoi(a)
	if err != nil {
		return -1, err
	}
	return i, nil
}
