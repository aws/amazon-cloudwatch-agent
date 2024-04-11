// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2

import (
	"context"

	"github.com/aws/aws-sdk-go/aws/client"
)

// Metadata is a set of information about the EC2 instance.
type Metadata struct {
	AccountID    string
	Hostname     string
	ImageID      string
	InstanceID   string
	InstanceType string
	PrivateIP    string
	Region       string
}

type MetadataProviderConfig struct {
	// IMDSv2Retries is the number of retries the IMDSv2 MetadataProvider will make before it errors out.
	IMDSv2Retries int
}

// MetadataProvider provides functions to get EC2 Metadata and the hostname.
type MetadataProvider interface {
	Get(ctx context.Context) (*Metadata, error)
	Hostname(ctx context.Context) (string, error)
	ID() string
}

func NewMetadataProvider(configProvider client.ConfigProvider, config MetadataProviderConfig) MetadataProvider {
	return newChainMetadataProvider(
		[]MetadataProvider{
			newIMDSv2MetadataProvider(configProvider, config.IMDSv2Retries),
			newIMDSv1MetadataProvider(configProvider),
			newDescribeInstancesMetadataProvider(configProvider),
		},
	)
}
