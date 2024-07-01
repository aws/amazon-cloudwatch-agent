// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resourcestore

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
)

type mockMetadataProvider struct {
	InstanceIdentityDocument *ec2metadata.EC2InstanceIdentityDocument
	Tags                     string
	TagValue                 string
}

type mockSTSClient struct {
	stsiface.STSAPI
}

func (ms *mockSTSClient) GetCallerIdentity(*sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{Account: aws.String("123456789")}, nil
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

func TestResourceStore_LogFiles(t *testing.T) {
	tests := []struct {
		name         string
		logFileInput map[string]ServiceAttribute
		want         map[string]ServiceAttribute
	}{
		{
			name:         "happypath",
			logFileInput: map[string]ServiceAttribute{"/opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log": {"cloudwatch-agent", "", "ec2:test"}},
			want:         map[string]ServiceAttribute{"/opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log": {"cloudwatch-agent", "", "ec2:test"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ResourceStore{
				serviceprovider: serviceprovider{
					logFiles: tt.logFileInput,
				},
			}
			if got := r.serviceprovider.logFiles; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("logFiles() = %v, want %v", got, tt.want)
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
		mode            string
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
				ServiceNameSourceKey: aws.String(ClientIamRole),
				ASGKey:               aws.String("test-asg"),
				InstanceIDKey:        aws.String("i-123456789"),
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
				ServiceNameSourceKey: aws.String(ResourceTags),
				ASGKey:               aws.String("test-asg"),
				InstanceIDKey:        aws.String("i-123456789"),
			},
		},
		{
			name: "HappyPath_TagServiceName",
			fields: fields{
				mode: config.ModeEC2,
			},
			want: map[string]*string{
				PlatformType: aws.String(EC2PlatForm),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ResourceStore{
				ec2Info:         tt.fields.ec2Info,
				serviceprovider: tt.fields.serviceprovider,
				mode:            tt.fields.mode,
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
			assert.Equalf(t, tt.want, r.createServiceKeyAttributes(""), "createServiceKeyAttributes()")
		})
	}
}

func TestResourceStore_shouldReturnRID(t *testing.T) {
	type fields struct {
		metadataprovider ec2metadataprovider.MetadataProvider
		stsClient        stsiface.STSAPI
		nativeCredential client.ConfigProvider
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "HappyPath_AccountIDMatches",
			fields: fields{
				metadataprovider: &mockMetadataProvider{
					InstanceIdentityDocument: &ec2metadata.EC2InstanceIdentityDocument{
						AccountID: "123456789"},
				},
				stsClient:        &mockSTSClient{},
				nativeCredential: &session.Session{},
			},
			want: true,
		},
		{
			name: "HappyPath_AccountIDMismatches",
			fields: fields{
				metadataprovider: &mockMetadataProvider{
					InstanceIdentityDocument: &ec2metadata.EC2InstanceIdentityDocument{
						AccountID: "987654321"},
				},
				stsClient:        &mockSTSClient{},
				nativeCredential: &session.Session{},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ResourceStore{
				metadataprovider: tt.fields.metadataprovider,
				stsClient:        tt.fields.stsClient,
				nativeCredential: tt.fields.nativeCredential,
			}
			assert.Equalf(t, tt.want, r.shouldReturnRID(), "shouldReturnRID()")
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

func TestAddServiceKeyAttributeToLogFilesMap(t *testing.T) {
	rs := &ResourceStore{
		metadataprovider: &mockMetadataProvider{
			InstanceIdentityDocument: &ec2metadata.EC2InstanceIdentityDocument{
				AccountID: "987654321"},
		},
		serviceprovider: serviceprovider{logFiles: map[string]ServiceAttribute{}},
	}
	key := "/opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log"
	rs.AddServiceAttrEntryToResourceStore(key, "test", "ec2:test")

	expected := &ResourceStore{
		serviceprovider: serviceprovider{
			iamRole:  "test-role",
			logFiles: map[string]ServiceAttribute{"/opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log": {ServiceName: "test", ServiceNameSource: AgentConfig, Environment: "ec2:test"}},
		},
	}

	assert.Equal(t, true, reflect.DeepEqual(rs.serviceprovider.logFiles, expected.serviceprovider.logFiles))
}
