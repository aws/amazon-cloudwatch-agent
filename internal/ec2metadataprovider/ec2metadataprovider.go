// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2metadataprovider

import (
	"context"
	"io"
	"strings"

	override "github.com/amazon-contributing/opentelemetry-collector-contrib/override/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"go.uber.org/zap"
)

type MetadataProvider interface {
	Get(ctx context.Context) (imds.InstanceIdentityDocument, error)
	Hostname(ctx context.Context) (string, error)
	InstanceID(ctx context.Context) (string, error)
	InstanceTags(ctx context.Context) ([]string, error)
	ClientIAMRole(ctx context.Context) (string, error)
	InstanceTagValue(ctx context.Context, tagKey string) (string, error)
}

type metadataClient struct {
	client *override.IMDSClient
}

var _ MetadataProvider = (*metadataClient)(nil)

// NewMetadataProvider returns a MetadataProvider backed by the shared IMDS client.
// logger may be nil, in which case IMDS retry and fallback decisions are not logged.
func NewMetadataProvider(cfg aws.Config, logger *zap.Logger, retries int) MetadataProvider {
	return newMetadataProvider(cfg, logger, retries)
}

func newMetadataProvider(cfg aws.Config, logger *zap.Logger, retries int, optFns ...func(*imds.Options)) MetadataProvider {
	return &metadataClient{
		client: override.NewIMDSClientFromConfig(cfg, logger, retries, optFns...),
	}
}

// NewMetadataProviderWithoutConfig returns a MetadataProvider that does not depend on an
// aws.Config, for callers that run before one can be loaded. The IMDSv1 opt-out is resolved
// from the environment only.
func NewMetadataProviderWithoutConfig(logger *zap.Logger, retries int, optFns ...func(*imds.Options)) MetadataProvider {
	return &metadataClient{
		client: override.NewIMDSClient(logger, retries, optFns...),
	}
}

func (c *metadataClient) InstanceID(ctx context.Context) (string, error) {
	return c.getMetadata(ctx, "instance-id")
}

func (c *metadataClient) Hostname(ctx context.Context) (string, error) {
	return c.getMetadata(ctx, "hostname")
}

func (c *metadataClient) ClientIAMRole(ctx context.Context) (string, error) {
	return c.getMetadata(ctx, "iam/security-credentials")
}

func (c *metadataClient) InstanceTags(ctx context.Context) ([]string, error) {
	tags, err := c.getMetadata(ctx, "tags/instance")
	if err != nil {
		return nil, err
	}
	return strings.Fields(tags), nil
}

func (c *metadataClient) InstanceTagValue(ctx context.Context, tagKey string) (string, error) {
	return c.getMetadata(ctx, "tags/instance/"+tagKey)
}

func (c *metadataClient) Get(ctx context.Context) (imds.InstanceIdentityDocument, error) {
	out, err := c.client.GetInstanceIdentityDocument(ctx, &imds.GetInstanceIdentityDocumentInput{})
	if err != nil {
		return imds.InstanceIdentityDocument{}, err
	}
	return out.InstanceIdentityDocument, nil
}

func (c *metadataClient) getMetadata(ctx context.Context, path string) (string, error) {
	out, err := c.client.GetMetadata(ctx, &imds.GetMetadataInput{Path: path})
	if err != nil {
		return "", err
	}
	content, err := io.ReadAll(out.Content)
	if err != nil {
		return "", err
	}
	return string(content), nil
}
