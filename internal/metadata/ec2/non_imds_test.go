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

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
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

func TestDescribeInstanceProvider(t *testing.T) {
	t.Setenv(envconfig.HostName, "")
	testErr := errors.New("test")
	testCases := map[string]struct {
		hostnameFn      func() (string, error)
		reservations    []*ec2.Reservation
		clientErr       error
		wantHostname    string
		wantMetadata    *Metadata
		wantHostnameErr error
		wantGetErr      error
	}{
		"WithHostname/PrivateIP": {
			hostnameFn: func() (string, error) {
				return "ip-10-24-34-0.ec2.internal", nil
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
				Region:       "us-east-1",
			},
			wantHostname: "ip-10-24-34-0.ec2.internal",
		},
		"WithHostname/ResourceName": {
			hostnameFn: func() (string, error) {
				return "i-0123456789abcdef.us-west-2.compute.internal", nil
			},
			reservations: []*ec2.Reservation{
				{
					Instances: []*ec2.Instance{
						{
							ImageId:          aws.String("image-id"),
							InstanceId:       aws.String("i-0123456789abcdef"),
							InstanceType:     aws.String("instance-type"),
							PrivateIpAddress: aws.String("private-ip"),
						},
					},
					OwnerId: aws.String("owner-id"),
				},
			},
			wantMetadata: &Metadata{
				AccountID:    "owner-id",
				ImageID:      "image-id",
				InstanceID:   "i-0123456789abcdef",
				InstanceType: "instance-type",
				PrivateIP:    "private-ip",
				Region:       "us-west-2",
			},
			wantHostname: "i-0123456789abcdef.us-west-2.compute.internal",
		},
		"WithHostname/Unsupported": {
			hostnameFn: func() (string, error) {
				return "hello.us-east-1.amazon.com", nil
			},
			wantHostname: "hello.us-east-1.amazon.com",
			wantGetErr:   errUnsupportedHostname,
		},
		"WithHostname/InvalidPrefix": {
			hostnameFn: func() (string, error) {
				return "other-prefix.us-west-2.compute.internal", nil
			},
			wantHostname: "other-prefix.us-west-2.compute.internal",
			wantGetErr:   errUnsupportedFilter,
		},
		"WithHostname/Error": {
			hostnameFn: func() (string, error) {
				return "", testErr
			},
			wantHostname:    "",
			wantHostnameErr: testErr,
			wantGetErr:      testErr,
		},
		"WithClient/Error": {
			hostnameFn: func() (string, error) {
				return "i-0123456789abcdef.us-west-2.compute.internal", nil
			},
			clientErr:    testErr,
			wantHostname: "i-0123456789abcdef.us-west-2.compute.internal",
			wantGetErr:   testErr,
		},
		"WithClient/NoReservations": {
			hostnameFn: func() (string, error) {
				return "i-0123456789abcdef.us-west-2.compute.internal", nil
			},
			reservations: []*ec2.Reservation{},
			wantHostname: "i-0123456789abcdef.us-west-2.compute.internal",
			wantGetErr:   errReservationCount,
		},
		"WithClient/NoInstances": {
			hostnameFn: func() (string, error) {
				return "i-0123456789abcdef.us-west-2.compute.internal", nil
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
			p.osHostname = testCase.hostnameFn
			hostname, err := p.Hostname(ctx)
			assert.ErrorIs(t, err, testCase.wantHostnameErr)
			assert.Equal(t, testCase.wantHostname, hostname)
			metadata, err := p.Get(ctx)
			assert.ErrorIs(t, err, testCase.wantGetErr)
			assert.Equal(t, testCase.wantMetadata, metadata)
		})
	}
}
