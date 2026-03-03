// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package azure

import (
	"context"

	azureprovider "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/metadataproviders/azure"

	"github.com/aws/amazon-cloudwatch-agent/internal/cloudprovider"
)

type Provider struct {
	metadata *azureprovider.ComputeMetadata
}

func NewProvider(ctx context.Context) (*Provider, error) {
	md, err := azureprovider.NewProvider().Metadata(ctx)
	if err != nil {
		return nil, err
	}
	return &Provider{metadata: md}, nil
}

func (p *Provider) Region() string                             { return p.metadata.Location }
func (p *Provider) InstanceID() string                         { return p.metadata.VMID }
func (p *Provider) Hostname() string                           { return p.metadata.Name }
func (p *Provider) InstanceType() string                       { return p.metadata.VMSize }
func (p *Provider) ImageID() string                            { return "" }
func (p *Provider) AccountID() string                          { return p.metadata.SubscriptionID }
func (p *Provider) PrivateIP() string                          { return "" }
func (p *Provider) CloudProvider() cloudprovider.CloudProvider { return cloudprovider.Azure }
