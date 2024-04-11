// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"
)

type imdsVersion string

const (
	IMDSv1 imdsVersion = "IMDSv1"
	IMDSv2 imdsVersion = "IMDSv2"

	metadataKeyHostname = "hostname"
)

type ec2MetadataClient interface {
	GetMetadataWithContext(ctx aws.Context, p string) (string, error)
	GetInstanceIdentityDocumentWithContext(ctx aws.Context) (ec2metadata.EC2InstanceIdentityDocument, error)
}

type imdsMetadataProvider struct {
	version imdsVersion
	svc     ec2MetadataClient
}

var _ MetadataProvider = (*imdsMetadataProvider)(nil)

func newIMDSv2MetadataProvider(configProvider client.ConfigProvider, retries int) *imdsMetadataProvider {
	return newIMDSProvider(IMDSv2, configProvider, &aws.Config{
		LogLevel:                  configaws.SDKLogLevel(),
		Logger:                    configaws.SDKLogger{},
		Retryer:                   retryer.NewIMDSRetryer(retries),
		EC2MetadataEnableFallback: aws.Bool(false),
	})
}

func newIMDSv1MetadataProvider(configProvider client.ConfigProvider) *imdsMetadataProvider {
	return newIMDSProvider(IMDSv1, configProvider, &aws.Config{
		LogLevel: configaws.SDKLogLevel(),
		Logger:   configaws.SDKLogger{},
	})
}

func newIMDSProvider(version imdsVersion, configProvider client.ConfigProvider, config *aws.Config) *imdsMetadataProvider {
	return &imdsMetadataProvider{
		svc:     ec2metadata.New(configProvider, config),
		version: version,
	}
}

func (p *imdsMetadataProvider) ID() string {
	return string(p.version)
}

// Hostname more information on API: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instancedata-data-retrieval.html#instance-metadata-ex-2
func (p *imdsMetadataProvider) Hostname(ctx context.Context) (string, error) {
	hostname, err := p.svc.GetMetadataWithContext(ctx, metadataKeyHostname)
	if err != nil {
		return "", err
	}
	if p.version == IMDSv1 {
		agent.UsageFlags().Set(agent.FlagIMDSFallbackSuccess)
	}
	return hostname, nil
}

// Get more information on API: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-identity-documents.html
func (p *imdsMetadataProvider) Get(ctx context.Context) (*Metadata, error) {
	instanceDocument, err := p.svc.GetInstanceIdentityDocumentWithContext(ctx)
	if err != nil {
		return nil, err
	}
	if p.version == IMDSv1 {
		agent.UsageFlags().Set(agent.FlagIMDSFallbackSuccess)
	}
	return fromInstanceIdentityDocument(instanceDocument), nil
}

func fromInstanceIdentityDocument(document ec2metadata.EC2InstanceIdentityDocument) *Metadata {
	return &Metadata{
		AccountID:    document.AccountID,
		ImageID:      document.ImageID,
		InstanceID:   document.InstanceID,
		InstanceType: document.InstanceType,
		PrivateIP:    document.PrivateIP,
		Region:       document.Region,
	}
}
