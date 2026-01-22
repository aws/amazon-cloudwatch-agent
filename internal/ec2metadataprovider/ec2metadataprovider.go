// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2metadataprovider

import (
	"context"
	"io"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
	"github.com/aws/amazon-cloudwatch-agent/internal/retryer/v2"
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
	// v2Client has fallback disabled, so it only tries to call IMDSv2.
	v2Client *imds.Client
	// v1Client has fallback enabled, so it will try to get the IMDSv2 token first and on failure will use IMDSv1.
	v1Client *imds.Client
}

var _ MetadataProvider = (*metadataClient)(nil)

func NewMetadataProvider(cfg aws.Config, retries int) MetadataProvider {
	return newMetadataProvider(cfg, retries)
}

func newMetadataProvider(cfg aws.Config, retries int, optFns ...func(*imds.Options)) MetadataProvider {
	v2Options := append(optFns, func(o *imds.Options) {
		o.Retryer = retryer.NewIMDSRetryer(retries)
		o.EnableFallback = aws.FalseTernary
	})
	v1Options := append(optFns, func(o *imds.Options) {
		o.EnableFallback = aws.TrueTernary
	})
	return &metadataClient{
		v2Client: imds.NewFromConfig(cfg, v2Options...),
		v1Client: imds.NewFromConfig(cfg, v1Options...),
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
	return withMetadataFallbackRetry(c, func(client *imds.Client) (imds.InstanceIdentityDocument, error) {
		out, err := client.GetInstanceIdentityDocument(ctx, &imds.GetInstanceIdentityDocumentInput{})
		if err != nil {
			return imds.InstanceIdentityDocument{}, err
		}
		return out.InstanceIdentityDocument, nil
	})
}

func (c *metadataClient) getMetadata(ctx context.Context, path string) (string, error) {
	return withMetadataFallbackRetry(c, func(client *imds.Client) (string, error) {
		out, err := client.GetMetadata(ctx, &imds.GetMetadataInput{
			Path: path,
		})
		if err != nil {
			return "", err
		}
		content, err := io.ReadAll(out.Content)
		if err != nil {
			return "", err
		}
		return string(content), nil
	})
}

// withMetadataFallbackRetry each fn call will first try the IMDS v2 client before falling back and retrying with the
// IMDS v1 client.
func withMetadataFallbackRetry[T any](c *metadataClient, fn func(*imds.Client) (T, error)) (T, error) {
	result, err := fn(c.v2Client)
	if err != nil {
		log.Printf("D! Could not perform operation without IMDS v1 fallback enabled. Enabling fallback.")
		result, err = fn(c.v1Client)
		if err == nil {
			agent.UsageFlags().Set(agent.FlagIMDSFallbackSuccess)
		}
	}
	return result, err
}
