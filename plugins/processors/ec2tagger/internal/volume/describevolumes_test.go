// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package volume

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/stretchr/testify/assert"
)

// construct the return results for the mocked DescribeTags api
var (
	device1           = "sda1"
	volumeId1         = "vol-0303a1cc896c42d28"
	volumeAttachment1 = ec2.VolumeAttachment{Device: aws.String("/dev/sda1"), VolumeId: aws.String(volumeId1)}
	availabilityZone  = "us-east-1a"
	volume1           = ec2.Volume{
		Attachments:      []*ec2.VolumeAttachment{&volumeAttachment1},
		AvailabilityZone: &availabilityZone,
	}
)

var (
	device2           = "xvdf"
	volumeId2         = "vol-0c241693efb58734a"
	volumeAttachment2 = ec2.VolumeAttachment{Device: aws.String("/dev/xvdf"), VolumeId: aws.String(volumeId2)}
	volume2           = ec2.Volume{
		Attachments:      []*ec2.VolumeAttachment{&volumeAttachment2},
		AvailabilityZone: &availabilityZone,
	}
)

var (
	device3           = "xvdda"
	volumeId3         = "vol-09ada5ca79a65cdd2"
	volumeAttachment3 = ec2.VolumeAttachment{Device: aws.String("xvdda"), VolumeId: aws.String(volumeId3)}
	volume3           = ec2.Volume{
		Attachments:      []*ec2.VolumeAttachment{&volumeAttachment3},
		AvailabilityZone: &availabilityZone,
	}
)

type mockEC2Client struct {
	ec2iface.EC2API

	callCount int
	err       error
}

func (m *mockEC2Client) DescribeVolumes(input *ec2.DescribeVolumesInput) (*ec2.DescribeVolumesOutput, error) {
	m.callCount++

	if m.err != nil {
		return nil, m.err
	}

	if input.NextToken == nil {
		return &ec2.DescribeVolumesOutput{
			NextToken: &device2,
			Volumes:   []*ec2.Volume{&volume1, &volume2},
		}, nil
	}
	return &ec2.DescribeVolumesOutput{
		NextToken: nil,
		Volumes:   []*ec2.Volume{&volume3},
	}, nil
}

func TestDescribeVolumesProvider(t *testing.T) {
	ec2Client := &mockEC2Client{}
	p := newDescribeVolumesProvider(ec2Client, "")
	got, err := p.DeviceToSerialMap()
	assert.NoError(t, err)
	assert.Equal(t, 2, ec2Client.callCount)
	want := map[string]string{device1: volumeId1, device2: volumeId2, device3: volumeId3}
	assert.Equal(t, want, got)
	ec2Client.err = errors.New("test")
	ec2Client.callCount = 0
	got, err = p.DeviceToSerialMap()
	assert.Error(t, err)
	assert.Equal(t, 1, ec2Client.callCount)
	assert.Nil(t, got)
}
