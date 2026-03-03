// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudmetadata

import "github.com/aws/amazon-cloudwatch-agent/internal/cloudprovider"

// Provider is a cloud-agnostic interface for instance metadata.
type Provider interface {
	Region() string
	InstanceID() string
	Hostname() string
	InstanceType() string
	ImageID() string
	AccountID() string
	PrivateIP() string
	CloudProvider() cloudprovider.CloudProvider
}
