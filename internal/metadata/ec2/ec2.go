// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2

import (
	"context"

	"github.com/aws/aws-sdk-go/aws/client"
)

// Metadata is a set of information about the EC2 instance.
type Metadata struct {
	AccountID        string
	AvailabilityZone string
	Hostname         string
	ImageID          string
	InstanceID       string
	InstanceType     string
	PrivateIP        string
	Region           string
}

// MetadataProvider provides functions to get EC2 Metadata and the hostname.
type MetadataProvider interface {
	Get(ctx context.Context) (*Metadata, error)
	Hostname(ctx context.Context) (string, error)
	ID() string
}

func NewMetadataProvider(configProvider client.ConfigProvider, options ...Option) MetadataProvider {
	cfg := DefaultConfig().WithOptions(options...)
	return newChainMetadataProvider(
		[]MetadataProvider{
			newIMDSv2MetadataProvider(configProvider, cfg.IMDSv2Retries),
			newIMDSv1MetadataProvider(configProvider),
			newDescribeInstancesMetadataProvider(configProvider),
		},
	)
}
