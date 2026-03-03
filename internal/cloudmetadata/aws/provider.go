// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/metadataproviders/ec2"

	"github.com/aws/amazon-cloudwatch-agent/internal/cloudprovider"
)

type Provider struct {
	region       string
	instanceID   string
	hostname     string
	instanceType string
	imageID      string
	accountID    string
	privateIP    string
}

func NewProvider(ctx context.Context) (*Provider, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	mdProvider := ec2.NewProvider(cfg)

	doc, err := mdProvider.Get(ctx)
	if err != nil {
		return nil, err
	}

	// Hostname is best-effort; empty string is acceptable.
	hostname, _ := mdProvider.Hostname(ctx)

	return &Provider{
		region:       doc.Region,
		instanceID:   doc.InstanceID,
		hostname:     hostname,
		instanceType: doc.InstanceType,
		imageID:      doc.ImageID,
		accountID:    doc.AccountID,
		privateIP:    doc.PrivateIP,
	}, nil
}

func (p *Provider) Region() string                             { return p.region }
func (p *Provider) InstanceID() string                         { return p.instanceID }
func (p *Provider) Hostname() string                           { return p.hostname }
func (p *Provider) InstanceType() string                       { return p.instanceType }
func (p *Provider) ImageID() string                            { return p.imageID }
func (p *Provider) AccountID() string                          { return p.accountID }
func (p *Provider) PrivateIP() string                          { return p.privateIP }
func (p *Provider) CloudProvider() cloudprovider.CloudProvider { return cloudprovider.AWS }
