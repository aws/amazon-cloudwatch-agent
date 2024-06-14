// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resourcestore

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
)

type mockServiceNameEC2Client struct {
	ec2iface.EC2API
}

// construct the return results for the mocked DescribeTags api
var (
	tagKeyService = "service"
	tagValService = "test-service"
	tagDesService = ec2.TagDescription{Key: &tagKeyService, Value: &tagValService}
)

func (m *mockServiceNameEC2Client) DescribeTags(*ec2.DescribeTagsInput) (*ec2.DescribeTagsOutput, error) {
	testTags := ec2.DescribeTagsOutput{
		NextToken: nil,
		Tags:      []*ec2.TagDescription{&tagDesService},
	}
	return &testTags, nil
}

func Test_serviceprovider_startServiceProvider(t *testing.T) {
	type args struct {
		metadataProvider ec2metadataprovider.MetadataProvider
		ec2Client        ec2iface.EC2API
	}
	tests := []struct {
		name    string
		args    args
		wantIAM string
		wantTag string
	}{
		{
			name: "HappyPath_AllServiceNames",
			args: args{
				metadataProvider: &mockMetadataProvider{
					InstanceIdentityDocument: &ec2metadata.EC2InstanceIdentityDocument{
						InstanceID: "i-123456789"},
				},
				ec2Client: &mockServiceNameEC2Client{},
			},
			wantIAM: "TestRole",
			wantTag: "test-service",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := serviceprovider{
				metadataProvider: tt.args.metadataProvider,
				ec2Provider: func(s string) ec2iface.EC2API {
					return tt.args.ec2Client
				},
			}
			s.startServiceProvider()
			time.Sleep(1 * time.Second)
			assert.Equal(t, tt.wantIAM, s.iamRole)
			assert.Equal(t, tt.wantTag, s.ec2TagServiceName)
		})
	}
}

func Test_serviceprovider_ServiceName(t *testing.T) {
	type fields struct {
		iamRole string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "HappyPath_IAMServiceName",
			fields: fields{
				iamRole: "MockIAM",
			},
			want: "MockIAM",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &serviceprovider{
				iamRole: tt.fields.iamRole,
			}
			assert.Equal(t, tt.want, s.ServiceName())
		})
	}
}

func Test_serviceprovider_getIAMRole(t *testing.T) {
	type fields struct {
		metadataProvider ec2metadataprovider.MetadataProvider
		ec2API           ec2iface.EC2API
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "Happypath_MockMetadata",
			fields: fields{
				metadataProvider: &mockMetadataProvider{},
				ec2API:           &mockServiceNameEC2Client{},
			},
			want: "TestRole",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := serviceprovider{
				metadataProvider: tt.fields.metadataProvider,
				ec2API:           tt.fields.ec2API,
			}
			s.getIAMRole()
			assert.Equal(t, tt.want, s.iamRole)
		})
	}
}

func Test_serviceprovider_getEC2TagFilters(t *testing.T) {
	type fields struct {
		metadataProvider ec2metadataprovider.MetadataProvider
		ec2API           ec2iface.EC2API
	}
	tests := []struct {
		name    string
		fields  fields
		want    []*ec2.Filter
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "HappyPath_MatchTags",
			fields: fields{
				metadataProvider: &mockMetadataProvider{
					InstanceIdentityDocument: &ec2metadata.EC2InstanceIdentityDocument{
						InstanceID: "i-123456789"},
				},
				ec2API: &mockServiceNameEC2Client{},
			},
			want: []*ec2.Filter{
				{
					Name:   aws.String("resource-type"),
					Values: aws.StringSlice([]string{"instance"}),
				}, {
					Name:   aws.String("resource-id"),
					Values: aws.StringSlice([]string{"i-123456789"}),
				}, {
					Name:   aws.String("key"),
					Values: aws.StringSlice([]string{"service", "application", "app"}),
				},
			},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &serviceprovider{
				metadataProvider: tt.fields.metadataProvider,
				ec2API:           tt.fields.ec2API,
			}
			got, err := s.getEC2TagFilters()
			assert.NoError(t, err)
			assert.Equalf(t, tt.want, got, "getEC2TagFilters()")
		})
	}
}

func Test_serviceprovider_getEC2TagServiceName(t *testing.T) {
	type fields struct {
		metadataProvider ec2metadataprovider.MetadataProvider
		ec2API           ec2iface.EC2API
	}
	tests := []struct {
		name               string
		fields             fields
		wantTagServiceName string
	}{
		{
			name: "HappyPath_ServiceExists",
			fields: fields{
				metadataProvider: &mockMetadataProvider{
					InstanceIdentityDocument: &ec2metadata.EC2InstanceIdentityDocument{
						InstanceID: "i-123456789"},
				},
				ec2API: &mockServiceNameEC2Client{},
			},
			wantTagServiceName: "test-service",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &serviceprovider{
				metadataProvider: tt.fields.metadataProvider,
				ec2API:           tt.fields.ec2API,
			}
			s.getEC2TagServiceName()
			assert.Equal(t, tt.wantTagServiceName, s.ec2TagServiceName)
		})
	}
}
