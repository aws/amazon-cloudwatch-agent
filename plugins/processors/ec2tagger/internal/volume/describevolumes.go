// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package volume

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

const (
	// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/device_naming.html
	possibleAttachmentDevicePrefix = "/dev/"
)

type describeVolumesProvider struct {
	ec2Client  ec2iface.EC2API
	instanceID string
}

func newDescribeVolumesProvider(ec2Client ec2iface.EC2API, instanceID string) Provider {
	return &describeVolumesProvider{ec2Client: ec2Client, instanceID: instanceID}
}

func (p *describeVolumesProvider) DeviceToSerialMap() (map[string]string, error) {
	result := map[string]string{}
	input := &ec2.DescribeVolumesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("attachment.instance-id"),
				Values: aws.StringSlice([]string{p.instanceID}),
			},
		},
	}
	for {
		output, err := p.ec2Client.DescribeVolumes(input)
		if err != nil {
			return nil, fmt.Errorf("unable to describe volumes: %w", err)
		}
		for _, volume := range output.Volumes {
			for _, attachment := range volume.Attachments {
				if attachment.Device != nil && attachment.VolumeId != nil {
					result[strings.TrimPrefix(aws.StringValue(attachment.Device), possibleAttachmentDevicePrefix)] = aws.StringValue(attachment.VolumeId)
				}
			}
		}
		if output.NextToken == nil {
			break
		}
		input.SetNextToken(*output.NextToken)
	}
	return result, nil
}
