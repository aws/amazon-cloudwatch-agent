// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlp

import (
	"context"
	"time"

	override "github.com/amazon-contributing/opentelemetry-collector-contrib/override/aws"

	"github.com/aws/amazon-cloudwatch-agent/extension/entitystore"
	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
)

const ec2InferenceTimeout = 5 * time.Second

var EC2ServiceNameProvider = ec2ServiceName

func ec2ServiceName() string {
	ctx, cancel := context.WithTimeout(context.Background(), ec2InferenceTimeout)
	defer cancel()
	return entitystore.ServiceNameFromIMDS(ctx, ec2metadataprovider.NewMetadataProviderWithoutConfig(nil, override.GetDefaultRetryNumber()))
}
