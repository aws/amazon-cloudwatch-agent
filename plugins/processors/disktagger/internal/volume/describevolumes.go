// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package volume

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type describeVolumesProvider struct {
	ec2Client  ec2.DescribeVolumesAPIClient
	instanceID string
}

func newDescribeVolumesProvider(ec2Client ec2.DescribeVolumesAPIClient, instanceID string) Provider {
	return &describeVolumesProvider{ec2Client: ec2Client, instanceID: instanceID}
}

func (p *describeVolumesProvider) DeviceToSerialMap(ctx context.Context) (map[string]string, error) {
	result := map[string]string{}
	paginator := ec2.NewDescribeVolumesPaginator(p.ec2Client, &ec2.DescribeVolumesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("attachment.instance-id"),
				Values: []string{p.instanceID},
			},
		},
	})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("unable to describe volumes: %w", err)
		}
		for _, volume := range output.Volumes {
			for _, attachment := range volume.Attachments {
				if attachment.Device != nil && attachment.VolumeId != nil {
					result[*attachment.Device] = *attachment.VolumeId
				}
			}
		}
	}

	return result, nil
}
