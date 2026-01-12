// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/sdk/endpoints/awsrulesfn"
)

const (
	bjsPartition          = "aws-cn"
	pdtPartition          = "aws-us-gov"
	lckPartition          = "aws-iso-b"
	dcaPartition          = "aws-iso"
	classicFallbackRegion = "us-east-1"
	bjsFallbackRegion     = "cn-north-1"
	pdtFallbackRegion     = "us-gov-west-1"
	lckFallbackRegion     = "us-isob-east-1"
	dcaFallbackRegion     = "us-iso-east-1"
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
		if errors.As(err, &rde) {
			log.Println("D! The regional STS endpoint is deactivated and going to fall back to partitional STS endpoint")
			p.fallback = p.partitional
			return p.fallback.Retrieve(ctx)
		}
	}
	return credentials, err
}

func newStsCredentialsProvider(cfg aws.Config, roleARN string, region string) aws.CredentialsProvider {
	regionalCfg := cfg.Copy()
	regionalCfg.Region = region
	partitionalCfg := cfg.Copy()
	partitionalCfg.Region = getFallbackRegion(region)
	return &stsCredentialsProvider{
		regional:    stscreds.NewAssumeRoleProvider(newAssumeRoleClient(regionalCfg), roleARN),
		partitional: stscreds.NewAssumeRoleProvider(newAssumeRoleClient(partitionalCfg), roleARN),
	}
}

type withHeaders struct {
	headers map[string]string
}

var _ middleware.FinalizeMiddleware = (*withHeaders)(nil)

func (w *withHeaders) ID() string {
	return "withHeaders"
}

func (w *withHeaders) HandleFinalize(ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
	req, ok := in.Request.(*smithyhttp.Request)
	if !ok {
		return middleware.FinalizeOutput{}, middleware.Metadata{}, fmt.Errorf("unrecognized transport type %T", in.Request)
	}
	for k, v := range w.headers {
		req.Header.Set(k, v)
	}
	return next.HandleFinalize(ctx, in)
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
			o.APIOptions = append(o.APIOptions, func(s *middleware.Stack) error {
				return s.Finalize.Add(&withHeaders{
					headers: map[string]string{
						SourceArnHeaderKey:     sourceArn,
						SourceAccountHeaderKey: sourceAccount,
					},
				}, middleware.Before)
			})
		})
	}
	return sts.NewFromConfig(cfg, options...)
}

// Get the region in the partition where STS endpoint cannot be deactivated by customers which is used to fallback.
// NOTE: Some Regions are not enabled by default, such as the Asia Pacific Hong Kong Region. In that case, when you
// manually enable the Region, the regional STS endpoints will always be activated and cannot be deactivated.
// Refer to: https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_temp_enable-regions.html
func getFallbackRegion(region string) string {
	partition := getPartition(region)
	switch partition {
	case bjsPartition:
		return bjsFallbackRegion
	case pdtPartition:
		return pdtFallbackRegion
	case dcaPartition:
		return dcaFallbackRegion
	case lckPartition:
		return lckFallbackRegion
	default:
		return classicFallbackRegion
	}
}

// Get the partition information based on the region name
func getPartition(region string) string {
	partition := awsrulesfn.GetPartition(region)
	if partition != nil {
		return partition.Name
	}
	return ""
}
