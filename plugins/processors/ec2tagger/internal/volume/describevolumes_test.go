// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package volume

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/stretchr/testify/assert"
)

// construct the return results for the mocked DescribeTags api
var (
	device1           = "/dev/xvdc"
	volumeId1         = "vol-0303a1cc896c42d28"
	volumeAttachment1 = types.VolumeAttachment{Device: &device1, VolumeId: &volumeId1}
	availabilityZone  = "us-east-1a"
	volume1           = types.Volume{
		Attachments:      []types.VolumeAttachment{volumeAttachment1},
		AvailabilityZone: &availabilityZone,
	}
)

var (
	device2           = "/dev/xvdf"
	volumeId2         = "vol-0c241693efb58734a"
	volumeAttachment2 = types.VolumeAttachment{Device: &device2, VolumeId: &volumeId2}
	volume2           = types.Volume{
		Attachments:      []types.VolumeAttachment{volumeAttachment2},
		AvailabilityZone: &availabilityZone,
	}
)

type mockEC2Client struct {
	callCount int
	err       error
}

var _ ec2.DescribeVolumesAPIClient = (*mockEC2Client)(nil)

func (m *mockEC2Client) DescribeVolumes(_ context.Context, input *ec2.DescribeVolumesInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	m.callCount++

	if m.err != nil {
		return nil, m.err
	}

	if input.NextToken == nil {
		return &ec2.DescribeVolumesOutput{
			NextToken: &device2,
			Volumes:   []types.Volume{volume1},
		}, nil
	}
	return &ec2.DescribeVolumesOutput{
		NextToken: nil,
		Volumes:   []types.Volume{volume2},
	}, nil
}

func TestDescribeVolumesProvider(t *testing.T) {
	ec2Client := &mockEC2Client{}
	p := newDescribeVolumesProvider(ec2Client, "")
	got, err := p.DeviceToSerialMap(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, 2, ec2Client.callCount)
	want := map[string]string{device1: volumeId1, device2: volumeId2}
	assert.Equal(t, want, got)
	ec2Client.err = errors.New("test")
	ec2Client.callCount = 0
	got, err = p.DeviceToSerialMap(t.Context())
	assert.Error(t, err)
	assert.Equal(t, 1, ec2Client.callCount)
	assert.Nil(t, got)
}
