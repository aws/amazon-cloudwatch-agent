// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2tagger

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/util/ec2util"
)

type MetadataProvider interface {
	Get(ctx context.Context) (imds.InstanceIdentityDocument, error)
	Hostname(ctx context.Context) (string, error)
	InstanceID(ctx context.Context) (string, error)
}

type metadataClient struct {
	metadata *ec2util.Ec2Util
}

var _ MetadataProvider = (*metadataClient)(nil)

func NewMetadataProvider(metadata *ec2util.Ec2Util) MetadataProvider {
	return &metadataClient{
		metadata: metadata,
	}
}

func (c *metadataClient) InstanceID(ctx context.Context) (string, error) {
	if c.metadata.InstanceID == "" {
		return "", errors.New("could not get ec2 instance id")
	}
	return c.metadata.InstanceID, nil
}

func (c *metadataClient) Hostname(ctx context.Context) (string, error) {
	if c.metadata.Hostname == "" {
		return "", errors.New("could not get ec2 hostname")
	}
	return c.metadata.Hostname, nil
}

func (c *metadataClient) Get(ctx context.Context) (imds.InstanceIdentityDocument, error) {
	if c.metadata.InstanceDocument == nil {
		return imds.InstanceIdentityDocument{}, errors.New("could not get instance document")
	}
	return *c.metadata.InstanceDocument, nil
}
