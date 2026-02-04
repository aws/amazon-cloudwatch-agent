// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudmetadata

// CloudProvider represents the cloud platform
type CloudProvider int

const (
	CloudProviderUnknown CloudProvider = iota
	CloudProviderAWS
	CloudProviderAzure
)

// String returns the string representation of the cloud provider
func (c CloudProvider) String() string {
	switch c {
	case CloudProviderAWS:
		return "AWS"
	case CloudProviderAzure:
		return "Azure"
	default:
		return "Unknown"
	}
}
