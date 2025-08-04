// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsnvmereceiver

import "fmt"

// DeviceType represents the type of NVMe device (EBS or Instance Store)
type DeviceType int

const (
	// DeviceTypeUnknown represents an unknown or undetected device type
	DeviceTypeUnknown DeviceType = iota
	// DeviceTypeEBS represents an Amazon EBS NVMe device
	DeviceTypeEBS
	// DeviceTypeInstanceStore represents an EC2 Instance Store NVMe device
	DeviceTypeInstanceStore
)

// String returns the string representation of the device type
func (dt DeviceType) String() string {
	switch dt {
	case DeviceTypeEBS:
		return "ebs"
	case DeviceTypeInstanceStore:
		return "instance_store"
	case DeviceTypeUnknown:
		return "unknown"
	default:
		return fmt.Sprintf("invalid_device_type_%d", int(dt))
	}
}

// IsValid returns true if the device type is a valid known type
func (dt DeviceType) IsValid() bool {
	return dt == DeviceTypeEBS || dt == DeviceTypeInstanceStore
}

// ParseDeviceType parses a string into a DeviceType
func ParseDeviceType(s string) DeviceType {
	switch s {
	case "ebs":
		return DeviceTypeEBS
	case "instance_store":
		return DeviceTypeInstanceStore
	default:
		return DeviceTypeUnknown
	}
}

// MarshalText implements encoding.TextMarshaler for JSON/YAML serialization
func (dt DeviceType) MarshalText() ([]byte, error) {
	return []byte(dt.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler for JSON/YAML deserialization
func (dt *DeviceType) UnmarshalText(text []byte) error {
	*dt = ParseDeviceType(string(text))
	return nil
}
