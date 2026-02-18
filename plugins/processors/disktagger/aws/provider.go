// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/disktagger/internal/volume"
)

// Provider maps device names to EBS volume IDs.
type Provider struct {
	cache volume.Cache
}

func NewProvider(ec2Client ec2.DescribeVolumesAPIClient, instanceID string) *Provider {
	return &Provider{
		cache: volume.NewCache(volume.NewProvider(ec2Client, instanceID)),
	}
}

func (p *Provider) Refresh() error {
	return p.cache.Refresh()
}

// Serial returns the volume ID for a device name, with prefix matching.
func (p *Provider) Serial(devName string) string {
	return p.cache.Serial(devName)
}
