// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudprovider

// CloudProvider identifies the cloud platform.
type CloudProvider int

const (
	Unknown CloudProvider = iota
	AWS
	Azure
)

func (c CloudProvider) String() string {
	switch c {
	case AWS:
		return "aws"
	case Azure:
		return "azure"
	default:
		return "unknown"
	}
}
