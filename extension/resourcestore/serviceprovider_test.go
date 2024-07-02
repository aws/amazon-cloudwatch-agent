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

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
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
			done := make(chan struct{})
			s := serviceprovider{
				metadataProvider: tt.args.metadataProvider,
				ec2Provider: func(s string, config *configaws.CredentialConfig) ec2iface.EC2API {
					return tt.args.ec2Client
				},
				ec2API: tt.args.ec2Client,
				done:   done,
			}
			go s.startServiceProvider()
			time.Sleep(3 * time.Second)
			close(done)

			assert.Equal(t, tt.wantIAM, s.iamRole)
			assert.Equal(t, tt.wantTag, s.ec2TagServiceName)
		})
	}
}

func Test_serviceprovider_ServiceAttribute(t *testing.T) {
	type fields struct {
		iamRole           string
		ec2TagServiceName string
		logFiles          map[string]ServiceAttribute
	}
	tests := []struct {
		name            string
		fields          fields
		serviceProvider *serviceprovider
		want            ServiceAttribute
	}{
		{
			name: "HappyPath_IAMRole",
			fields: fields{
				iamRole: "TestRole",
			},
			want: ServiceAttribute{
				ServiceName:       "TestRole",
				ServiceNameSource: ClientIamRole,
			},
		},
		{
			name: "HappyPath_EC2TagServiceName",
			fields: fields{
				ec2TagServiceName: "tag-service",
			},
			want: ServiceAttribute{
				ServiceName:       "tag-service",
				ServiceNameSource: ResourceTags,
			},
		},
		{
			name: "HappyPath_AgentConfig",
			fields: fields{
				logFiles: map[string]ServiceAttribute{
					"test-file": {
						ServiceName:       "test-service",
						ServiceNameSource: AgentConfig,
						Environment:       "test-environment",
					},
				},
			},
			want: ServiceAttribute{
				ServiceName:       "test-service",
				ServiceNameSource: AgentConfig,
				Environment:       "test-environment",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &serviceprovider{
				iamRole:           tt.fields.iamRole,
				ec2TagServiceName: tt.fields.ec2TagServiceName,
				logFiles:          tt.fields.logFiles,
			}
			assert.Equalf(t, tt.want, s.ServiceAttribute("test-file"), "ServiceAttribute()")
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

func Test_refreshLoop(t *testing.T) {
	type fields struct {
		metadataProvider  ec2metadataprovider.MetadataProvider
		ec2API            ec2iface.EC2API
		iamRole           string
		ec2TagServiceName string
		refreshInterval   time.Duration
		oneTime           bool
	}
	type expectedInfo struct {
		iamRole           string
		ec2TagServiceName string
	}
	tests := []struct {
		name         string
		fields       fields
		expectedInfo expectedInfo
	}{
		{
			name: "HappyPath_CorrectRefresh",
			fields: fields{
				metadataProvider: &mockMetadataProvider{
					InstanceIdentityDocument: &ec2metadata.EC2InstanceIdentityDocument{
						InstanceID: "i-123456789"},
				},
				ec2API:            &mockServiceNameEC2Client{},
				iamRole:           "original-role",
				ec2TagServiceName: "original-tag-name",
				refreshInterval:   time.Millisecond,
			},
			expectedInfo: expectedInfo{
				iamRole:           "TestRole",
				ec2TagServiceName: "test-service",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			done := make(chan struct{})
			s := &serviceprovider{
				metadataProvider: tt.fields.metadataProvider,
				ec2API:           tt.fields.ec2API,
				ec2Provider: func(s string, config *configaws.CredentialConfig) ec2iface.EC2API {
					return tt.fields.ec2API
				},
				iamRole:           tt.fields.iamRole,
				ec2TagServiceName: tt.fields.ec2TagServiceName,
				done:              done,
			}
			go refreshLoop(done, s.getEC2TagServiceName, tt.fields.oneTime)
			go refreshLoop(done, s.getIAMRole, tt.fields.oneTime)
			time.Sleep(time.Second)
			close(done)
			assert.Equal(t, tt.expectedInfo.iamRole, s.iamRole)
			assert.Equal(t, tt.expectedInfo.ec2TagServiceName, s.ec2TagServiceName)
		})
	}
}
