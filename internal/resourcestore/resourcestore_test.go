// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resourcestore

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
)

type mockMetadataProvider struct {
	InstanceIdentityDocument *ec2metadata.EC2InstanceIdentityDocument
	Tags                     string
	TagValue                 string
}

func (m *mockMetadataProvider) Get(ctx context.Context) (ec2metadata.EC2InstanceIdentityDocument, error) {
	if m.InstanceIdentityDocument != nil {
		return *m.InstanceIdentityDocument, nil
	}
	return ec2metadata.EC2InstanceIdentityDocument{}, errors.New("No instance identity document")
}

func (m *mockMetadataProvider) Hostname(ctx context.Context) (string, error) {
	return "MockHostName", nil
}

func (m *mockMetadataProvider) InstanceID(ctx context.Context) (string, error) {
	return "MockInstanceID", nil
}

func (m *mockMetadataProvider) InstanceProfileIAMRole() (string, error) {
	return "arn:aws:iam::123456789:instance-profile/TestRole", nil
}

func (m *mockMetadataProvider) InstanceTags(ctx context.Context) (string, error) {
	return m.Tags, nil
}

func (m *mockMetadataProvider) InstanceTagValue(ctx context.Context, tagKey string) (string, error) {
	return m.TagValue, nil
}

func TestInitResourceStore(t *testing.T) {
	tests := []struct {
		name string
	}{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initResourceStore()
		})
	}
}

func TestResourceStore_EC2Info(t *testing.T) {
	tests := []struct {
		name         string
		ec2InfoInput ec2Info
		want         ec2Info
	}{
		{
			name: "happypath",
			ec2InfoInput: ec2Info{
				InstanceID:       "i-1234567890",
				AutoScalingGroup: "test-asg",
			},
			want: ec2Info{
				InstanceID:       "i-1234567890",
				AutoScalingGroup: "test-asg",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ResourceStore{
				ec2Info: tt.ec2InfoInput,
			}
			if got := r.EC2Info(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EC2Info() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceStore_EKSInfo(t *testing.T) {
	tests := []struct {
		name         string
		eksInfoInput eksInfo
		want         eksInfo
	}{
		{
			name:         "happypath",
			eksInfoInput: eksInfo{ClusterName: "test-cluster"},
			want:         eksInfo{ClusterName: "test-cluster"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ResourceStore{
				eksInfo: tt.eksInfoInput,
			}
			if got := r.EKSInfo(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EKSInfo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceStore_LogFiles(t *testing.T) {
	tests := []struct {
		name         string
		logFileInput map[string]string
		want         map[string]string
	}{
		{
			name:         "happypath",
			logFileInput: map[string]string{"/opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log": "cloudwatch-agent"},
			want:         map[string]string{"/opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log": "cloudwatch-agent"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ResourceStore{
				logFiles: tt.logFileInput,
			}
			if got := r.LogFiles(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LogFiles() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceStore_Mode(t *testing.T) {
	tests := []struct {
		name      string
		modeInput string
		want      string
	}{
		{name: "happypath", modeInput: "EC2", want: "EC2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ResourceStore{
				mode: tt.modeInput,
			}
			if got := r.Mode(); got != tt.want {
				t.Errorf("Mode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getRegion(t *testing.T) {
	tests := []struct {
		name             string
		metadataProvider ec2metadataprovider.MetadataProvider
		want             string
	}{
		{
			name: "HappyPath",
			metadataProvider: &mockMetadataProvider{
				InstanceIdentityDocument: &ec2metadata.EC2InstanceIdentityDocument{
					Region: "us-west-2"},
			},
			want: "us-west-2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getRegion(tt.metadataProvider)
			assert.NoError(t, err)
			assert.Equalf(t, tt.want, got, "getRegion(%v)", tt.metadataProvider)
		})
	}
}

func TestResourceStore_createAttributeMaps(t *testing.T) {
	type fields struct {
		ec2Info         ec2Info
		serviceprovider serviceprovider
	}
	tests := []struct {
		name   string
		fields fields
		want   map[string]*string
	}{
		{
			name: "HappyPath_IAMRole",
			fields: fields{
				ec2Info: ec2Info{
					InstanceID:       "i-123456789",
					AutoScalingGroup: "test-asg",
				},
				serviceprovider: serviceprovider{
					iamRole: "test-role",
				},
			},
			want: map[string]*string{
				ServieNameSourceKey: aws.String(ClientIamRole),
				ASGKey:              aws.String("test-asg"),
				InstanceIDKey:       aws.String("i-123456789"),
			},
		},
		{
			name: "HappyPath_TagServiceName",
			fields: fields{
				ec2Info: ec2Info{
					InstanceID:       "i-123456789",
					AutoScalingGroup: "test-asg",
				},
				serviceprovider: serviceprovider{
					ec2TagServiceName: "test-tag-service",
				},
			},
			want: map[string]*string{
				ServieNameSourceKey: aws.String(ResourceTags),
				ASGKey:              aws.String("test-asg"),
				InstanceIDKey:       aws.String("i-123456789"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ResourceStore{
				ec2Info:         tt.fields.ec2Info,
				serviceprovider: tt.fields.serviceprovider,
			}
			assert.Equalf(t, dereferenceMap(tt.want), dereferenceMap(r.createAttributeMaps()), "createAttributeMaps()")
		})
	}
}

func TestResourceStore_createServiceKeyAttributes(t *testing.T) {
	type fields struct {
		serviceprovider serviceprovider
	}
	tests := []struct {
		name   string
		fields fields
		want   *cloudwatchlogs.KeyAttributes
	}{
		{
			name: "HappyPath_",
			fields: fields{
				serviceprovider: serviceprovider{
					iamRole: "test-role",
				},
			},
			want: &cloudwatchlogs.KeyAttributes{
				Name: aws.String("test-role"),
				Type: aws.String(Service),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ResourceStore{
				serviceprovider: tt.fields.serviceprovider,
			}
			assert.Equalf(t, tt.want, r.createServiceKeyAttributes(), "createServiceKeyAttributes()")
		})
	}
}

func dereferenceMap(input map[string]*string) map[string]string {
	result := make(map[string]string)
	for k, v := range input {
		if v != nil {
			result[k] = *v
		} else {
			result[k] = ""
		}
	}
	return result
}
