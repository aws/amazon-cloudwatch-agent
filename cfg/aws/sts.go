// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"context"
	"errors"
	"log"
	"os"

	override "github.com/amazon-contributing/opentelemetry-collector-contrib/override/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
	smithymiddleware "github.com/aws/smithy-go/middleware"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/middleware"
)

type stsCredentialsProvider struct {
	fallback    aws.CredentialsProvider
	regional    aws.CredentialsProvider
	partitional aws.CredentialsProvider
}

var _ aws.CredentialsProvider = (*stsCredentialsProvider)(nil)

func (p *stsCredentialsProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	if p.fallback != nil {
		return p.fallback.Retrieve(ctx)
	}
	credentials, err := p.regional.Retrieve(ctx)
	if err != nil {
		var rde *types.RegionDisabledException
		if errors.As(err, &rde) && p.partitional != nil {
			log.Println("D! The regional STS endpoint is deactivated and going to fall back to partitional STS endpoint")
			p.fallback = p.partitional
			return p.fallback.Retrieve(ctx)
		}
	}
	return credentials, err
}

// newStsCredentialsProvider builds an assume-role provider against the regional STS
// endpoint, falling back to the partition's primary endpoint on RegionDisabledException.
// The fallback provider is skipped when the partition has no known primary region, so a
// partition newer than the shared partition data stays regional-only instead of retrying
// in the wrong partition.
func newStsCredentialsProvider(cfg aws.Config, roleARN string, region string) aws.CredentialsProvider {
	regionalCfg := cfg.Copy()
	regionalCfg.Region = region

	p := &stsCredentialsProvider{
		regional: stscreds.NewAssumeRoleProvider(newAssumeRoleClient(regionalCfg), roleARN),
	}

	// Unlike a regional endpoint, the primary region's STS endpoint cannot be deactivated by
	// customers. Refer to:
	// https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_temp_enable-regions.html
	if fallbackRegion := override.GetPartitionPrimaryRegion(region); fallbackRegion != "" {
		partitionalCfg := cfg.Copy()
		partitionalCfg.Region = fallbackRegion
		p.partitional = stscreds.NewAssumeRoleProvider(newAssumeRoleClient(partitionalCfg), roleARN)
	}

	return p
}

const (
	SourceArnHeaderKey     = "x-amz-source-arn"
	SourceAccountHeaderKey = "x-amz-source-account"
)

var newAssumeRoleClient = newStsClient

func newStsClient(cfg aws.Config) stscreds.AssumeRoleAPIClient {
	var options []func(*sts.Options)
	sourceAccount := os.Getenv(envconfig.AmzSourceAccount)
	sourceArn := os.Getenv(envconfig.AmzSourceArn)
	if sourceAccount != "" && sourceArn != "" {
		options = append(options, func(o *sts.Options) {
			o.APIOptions = append(o.APIOptions, func(s *smithymiddleware.Stack) error {
				return s.Build.Add(middleware.NewCustomHeaderMiddleware("ConfusedDeputyHeaders", map[string]string{
					SourceArnHeaderKey:     sourceArn,
					SourceAccountHeaderKey: sourceAccount,
				}), smithymiddleware.Before)
			})
		})
	}
	return sts.NewFromConfig(cfg, options...)
}
