// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2tagger

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
)

type MetadataProvider interface {
	Get(ctx context.Context) (ec2metadata.EC2InstanceIdentityDocument, error)
	Hostname(ctx context.Context) (string, error)
	InstanceID(ctx context.Context) (string, error)
}

type metadataClient struct {
	metadata *ec2metadata.EC2Metadata
}

var _ MetadataProvider = (*metadataClient)(nil)

func NewMetadataProvider(p client.ConfigProvider, cfgs ...*aws.Config) MetadataProvider {
	return &metadataClient{
		metadata: ec2metadata.New(p, cfgs...),
	}
}

func (c *metadataClient) InstanceID(ctx context.Context) (string, error) {
	return c.metadata.GetMetadataWithContext(ctx, "instance-id")
}

func (c *metadataClient) Hostname(ctx context.Context) (string, error) {
	return c.metadata.GetMetadataWithContext(ctx, "hostname")
}

func (c *metadataClient) Get(ctx context.Context) (ec2metadata.EC2InstanceIdentityDocument, error) {
	return c.metadata.GetInstanceIdentityDocumentWithContext(ctx)
}
