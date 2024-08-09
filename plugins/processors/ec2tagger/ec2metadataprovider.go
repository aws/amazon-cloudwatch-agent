// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2tagger

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
	instanceId, err := c.metadataFallbackDisabled.GetMetadataWithContext(ctx, "instance-id")
	if err != nil {
		log.Printf("D! could not get instance id without imds v1 fallback enable thus enable fallback")
		instanceInner, errorInner := c.metadataFallbackEnabled.GetMetadataWithContext(ctx, "instance-id")
		if errorInner == nil {
			agent.UsageFlags().Set(agent.FlagIMDSFallbackSuccess)
		}
		return instanceInner, errorInner
	}
	return instanceId, err
}

func (c *metadataClient) Hostname(ctx context.Context) (string, error) {
	hostname, err := c.metadataFallbackDisabled.GetMetadataWithContext(ctx, "hostname")
	if err != nil {
		log.Printf("D! could not get hostname without imds v1 fallback enable thus enable fallback")
		hostnameInner, errorInner := c.metadataFallbackEnabled.GetMetadataWithContext(ctx, "hostname")
		if errorInner == nil {
			agent.UsageFlags().Set(agent.FlagIMDSFallbackSuccess)
		}
		return hostnameInner, errorInner
	}
	return hostname, err
}

func (c *metadataClient) Get(ctx context.Context) (ec2metadata.EC2InstanceIdentityDocument, error) {
	instanceDocument, err := c.metadataFallbackDisabled.GetInstanceIdentityDocumentWithContext(ctx)
	if err != nil {
		log.Printf("D! could not get instance document without imds v1 fallback enable thus enable fallback")
		instanceDocumentInner, errorInner := c.metadataFallbackEnabled.GetInstanceIdentityDocumentWithContext(ctx)
		if errorInner == nil {
			agent.UsageFlags().Set(agent.FlagIMDSFallbackSuccess)
		}
		return instanceDocumentInner, errorInner
	}
	return instanceDocument, err
}
