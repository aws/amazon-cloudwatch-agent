// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2metadataprovider

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"
)

type MetadataProvider interface {
	Get(ctx context.Context) (ec2metadata.EC2InstanceIdentityDocument, error)
	Hostname(ctx context.Context) (string, error)
	InstanceID(ctx context.Context) (string, error)
	InstanceProfileIAMRole() (string, error)
}

type metadataClient struct {
	metadataFallbackDisabled *ec2metadata.EC2Metadata
	metadataFallbackEnabled  *ec2metadata.EC2Metadata
}

var _ MetadataProvider = (*metadataClient)(nil)

func NewMetadataProvider(p client.ConfigProvider, retries int) MetadataProvider {
	disableFallbackConfig := &aws.Config{
		LogLevel:                  configaws.SDKLogLevel(),
		Logger:                    configaws.SDKLogger{},
		Retryer:                   retryer.NewIMDSRetryer(retries),
		EC2MetadataEnableFallback: aws.Bool(false),
	}
	enableFallbackConfig := &aws.Config{
		LogLevel: configaws.SDKLogLevel(),
		Logger:   configaws.SDKLogger{},
	}
	return &metadataClient{
		metadataFallbackDisabled: ec2metadata.New(p, disableFallbackConfig),
		metadataFallbackEnabled:  ec2metadata.New(p, enableFallbackConfig),
	}
}

func (c *metadataClient) InstanceID(ctx context.Context) (string, error) {
	return withMetadataFallbackRetry(ctx, c, func(metadataClient *ec2metadata.EC2Metadata) (string, error) {
		return metadataClient.GetMetadataWithContext(ctx, "instance-id")
	})
}

func (c *metadataClient) Hostname(ctx context.Context) (string, error) {
	return withMetadataFallbackRetry(ctx, c, func(metadataClient *ec2metadata.EC2Metadata) (string, error) {
		return metadataClient.GetMetadataWithContext(ctx, "hostname")
	})
}

func (c *metadataClient) InstanceProfileIAMRole() (string, error) {
	return withMetadataFallbackRetry(context.Background(), c, func(metadataClient *ec2metadata.EC2Metadata) (string, error) {
		iamInfo, err := metadataClient.IAMInfo()
		if err != nil {
			return "", err
		}
		return iamInfo.InstanceProfileArn, nil
	})
}

func (c *metadataClient) Get(ctx context.Context) (ec2metadata.EC2InstanceIdentityDocument, error) {
	return withMetadataFallbackRetry(ctx, c, func(metadataClient *ec2metadata.EC2Metadata) (ec2metadata.EC2InstanceIdentityDocument, error) {
		return metadataClient.GetInstanceIdentityDocumentWithContext(ctx)
	})
}

func withMetadataFallbackRetry[T any](ctx context.Context, c *metadataClient, operation func(*ec2metadata.EC2Metadata) (T, error)) (T, error) {
	result, err := operation(c.metadataFallbackDisabled)
	if err != nil {
		log.Printf("D! could not perform operation without imds v1 fallback enable thus enable fallback")
		result, err = operation(c.metadataFallbackEnabled)
		if err == nil {
			agent.UsageFlags().Set(agent.FlagIMDSFallbackSuccess)
		}
	}
	return result, err
}
