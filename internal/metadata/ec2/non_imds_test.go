// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	awsmock "github.com/aws/aws-sdk-go/awstesting/mock"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/internal/metadata/host"
)

type mockEC2Client struct {
	ec2iface.EC2API
	reservations []*ec2.Reservation
	err          error
}

func (m *mockEC2Client) DescribeInstances(*ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.reservations == nil {
		return nil, errors.New("no reservations")
	}
	return &ec2.DescribeInstancesOutput{
		Reservations: m.reservations,
	}, nil
}

type mockHostMetadataProvider struct {
	hostname    string
	hostIP      string
	errHostname error
	errHostIP   error
}

func (m *mockHostMetadataProvider) Hostname() (string, error) {
	return m.hostname, m.errHostname
}

func (m *mockHostMetadataProvider) HostIP() (string, error) {
	return m.hostIP, m.errHostIP
}

func TestDescribeInstanceProvider(t *testing.T) {
	testErr := errors.New("test")
	testCases := map[string]struct {
		hostMetadataProvider host.MetadataProvider
		reservations         []*ec2.Reservation
		clientErr            error
		wantHostname         string
		wantMetadata         *Metadata
		wantHostnameErr      error
		wantGetErr           error
	}{
		"WithHostname/PrivateIP": {
			hostMetadataProvider: &mockHostMetadataProvider{
				hostname: "ip-10-24-34-0.ec2.internal",
			},
			reservations: []*ec2.Reservation{
				{
					Instances: []*ec2.Instance{
						{
							ImageId:          aws.String("image-id"),
							InstanceId:       aws.String("instance-id"),
							InstanceType:     aws.String("instance-type"),
							PrivateIpAddress: aws.String("10.24.34.0"),
							Placement: &ec2.Placement{
								AvailabilityZone: aws.String("us-east-1a"),
							},
						},
					},
					OwnerId: aws.String("owner-id"),
				},
			},
			wantMetadata: &Metadata{
				AccountID:        "owner-id",
				AvailabilityZone: "us-east-1a",
				ImageID:          "image-id",
				InstanceID:       "instance-id",
				InstanceType:     "instance-type",
				PrivateIP:        "10.24.34.0",
				Region:           "us-east-1",
			},
			wantHostname: "ip-10-24-34-0.ec2.internal",
		},
		"WithHostname/ResourceName": {
			hostMetadataProvider: &mockHostMetadataProvider{
				hostname: "i-0123456789abcdef.us-west-2.compute.internal",
			},
			reservations: []*ec2.Reservation{
				{
					Instances: []*ec2.Instance{
						{
							ImageId:          aws.String("image-id"),
							InstanceId:       aws.String("i-0123456789abcdef"),
							InstanceType:     aws.String("instance-type"),
							PrivateIpAddress: aws.String("private-ip"),
							Placement: &ec2.Placement{
								AvailabilityZone: aws.String("us-west-2a"),
							},
						},
					},
					OwnerId: aws.String("owner-id"),
				},
			},
			wantMetadata: &Metadata{
				AccountID:        "owner-id",
				AvailabilityZone: "us-west-2a",
				ImageID:          "image-id",
				InstanceID:       "i-0123456789abcdef",
				InstanceType:     "instance-type",
				PrivateIP:        "private-ip",
				Region:           "us-west-2",
			},
			wantHostname: "i-0123456789abcdef.us-west-2.compute.internal",
		},
		"WithHostname/Unsupported": {
			hostMetadataProvider: &mockHostMetadataProvider{
				hostname:  "hello.us-east-1.amazon.com",
				errHostIP: testErr,
			},
			wantHostname: "hello.us-east-1.amazon.com",
			wantGetErr:   errUnsupportedHostname,
		},
		"WithHostname/InvalidPrefix": {
			hostMetadataProvider: &mockHostMetadataProvider{
				hostname:  "invalid-prefix.us-west-2.compute.internal",
				errHostIP: testErr,
			},
			wantHostname: "invalid-prefix.us-west-2.compute.internal",
			wantGetErr:   errUnsupportedFilter,
		},
		"WithHostname/Error": {
			hostMetadataProvider: &mockHostMetadataProvider{
				errHostname: testErr,
				errHostIP:   testErr,
			},
			wantHostname:    "",
			wantHostnameErr: testErr,
			wantGetErr:      testErr,
		},
		"WithHostIP/WithAZ": {
			hostMetadataProvider: &mockHostMetadataProvider{
				hostname: "hello.us-east-1.amazon.com",
				hostIP:   "10.24.34.0",
			},
			reservations: []*ec2.Reservation{
				{
					Instances: []*ec2.Instance{
						{
							ImageId:          aws.String("image-id"),
							InstanceId:       aws.String("instance-id"),
							InstanceType:     aws.String("instance-type"),
							PrivateIpAddress: aws.String("10.24.34.0"),
							Placement: &ec2.Placement{
								AvailabilityZone: aws.String("us-east-1a"),
							},
						},
					},
					OwnerId: aws.String("owner-id"),
				},
			},
			wantMetadata: &Metadata{
				AccountID:        "owner-id",
				AvailabilityZone: "us-east-1a",
				ImageID:          "image-id",
				InstanceID:       "instance-id",
				InstanceType:     "instance-type",
				PrivateIP:        "10.24.34.0",
				Region:           "us-east-1",
			},
			wantHostname: "hello.us-east-1.amazon.com",
		},
		"WithHostIP/WithoutAZ": {
			hostMetadataProvider: &mockHostMetadataProvider{
				hostname: "hello.us-east-1.amazon.com",
				hostIP:   "10.24.34.0",
			},
			reservations: []*ec2.Reservation{
				{
					Instances: []*ec2.Instance{
						{
							ImageId:          aws.String("image-id"),
							InstanceId:       aws.String("instance-id"),
							InstanceType:     aws.String("instance-type"),
							PrivateIpAddress: aws.String("10.24.34.0"),
						},
					},
					OwnerId: aws.String("owner-id"),
				},
			},
			wantMetadata: &Metadata{
				AccountID:    "owner-id",
				ImageID:      "image-id",
				InstanceID:   "instance-id",
				InstanceType: "instance-type",
				PrivateIP:    "10.24.34.0",
			},
			wantHostname: "hello.us-east-1.amazon.com",
		},
		"WithClient/Error": {
			hostMetadataProvider: &mockHostMetadataProvider{
				hostname: "i-0123456789abcdef.us-west-2.compute.internal",
			},
			clientErr:    testErr,
			wantHostname: "i-0123456789abcdef.us-west-2.compute.internal",
			wantGetErr:   testErr,
		},
		"WithClient/NoReservations": {
			hostMetadataProvider: &mockHostMetadataProvider{
				hostname: "i-0123456789abcdef.us-west-2.compute.internal",
			},
			reservations: []*ec2.Reservation{},
			wantHostname: "i-0123456789abcdef.us-west-2.compute.internal",
			wantGetErr:   errReservationCount,
		},
		"WithClient/NoInstances": {
			hostMetadataProvider: &mockHostMetadataProvider{
				hostname: "i-0123456789abcdef.us-west-2.compute.internal",
			},
			reservations: []*ec2.Reservation{
				{OwnerId: aws.String("owner-id")},
			},
			wantHostname: "i-0123456789abcdef.us-west-2.compute.internal",
			wantGetErr:   errInstanceCount,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			p := newDescribeInstancesMetadataProvider(awsmock.Session)
			assert.Equal(t, "DescribeInstances", p.ID())
			mockClient := &mockEC2Client{
				reservations: testCase.reservations,
				err:          testCase.clientErr,
			}
			p.newEC2Client = func(_ client.ConfigProvider, configs ...*aws.Config) ec2iface.EC2API {
				return mockClient
			}
			p.hostMetadataProvider = testCase.hostMetadataProvider
			hostname, err := p.Hostname(ctx)
			assert.ErrorIs(t, err, testCase.wantHostnameErr)
			assert.Equal(t, testCase.wantHostname, hostname)
			metadata, err := p.Get(ctx)
			assert.ErrorIs(t, err, testCase.wantGetErr)
			assert.Equal(t, testCase.wantMetadata, metadata)
		})
	}
}
