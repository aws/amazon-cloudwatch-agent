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
	"github.com/stretchr/testify/mock"

	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
)

type mockServiceProvider struct {
	mock.Mock
}

func (s *mockServiceProvider) startServiceProvider() {}

func (s *mockServiceProvider) addEntryForLogGroup(logGroupName LogGroupName, serviceAttr ServiceAttribute) {
	s.Called(logGroupName, serviceAttr)
}

func (s *mockServiceProvider) addEntryForLogFile(logFileGlob LogFileGlob, serviceAttr ServiceAttribute) {
	s.Called(logFileGlob, serviceAttr)
}

func (s *mockServiceProvider) logFileServiceAttribute(glob LogFileGlob, name LogGroupName) ServiceAttribute {
	args := s.Called(glob, name)
	return args.Get(0).(ServiceAttribute)
}

type mockSTSClient struct {
	stsiface.STSAPI
	accountId string
}

func (ms *mockSTSClient) GetCallerIdentity(*sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{Account: aws.String(ms.accountId)}, nil
}

type mockMetadataProvider struct {
	InstanceIdentityDocument *ec2metadata.EC2InstanceIdentityDocument
	Tags                     string
	TagValue                 string
}

func mockMetadataProviderWithAccountId(accountId string) *mockMetadataProvider {
	return &mockMetadataProvider{
		InstanceIdentityDocument: &ec2metadata.EC2InstanceIdentityDocument{
			AccountID: accountId,
		},
	}
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
		ec2Info ec2Info
		mode    string
	}
	tests := []struct {
		name   string
		fields fields
		want   map[string]*string
	}{
		{
			name: "HappyPath",
			fields: fields{
				ec2Info: ec2Info{
					InstanceID:       "i-123456789",
					AutoScalingGroup: "test-asg",
				},
				mode: config.ModeEC2,
			},
			want: map[string]*string{
				ASGKey:        aws.String("test-asg"),
				InstanceIDKey: aws.String("i-123456789"),
				PlatformType:  aws.String(EC2PlatForm),
			},
		},
		{
			name: "HappyPath_AsgMissing",
			fields: fields{
				ec2Info: ec2Info{
					InstanceID: "i-123456789",
				},
				mode: config.ModeEC2,
			},
			want: map[string]*string{
				InstanceIDKey: aws.String("i-123456789"),
				PlatformType:  aws.String(EC2PlatForm),
			},
		},
		{
			name: "HappyPath_InstanceIdAndAsgMissing",
			fields: fields{
				mode: config.ModeEC2,
			},
			want: map[string]*string{
				PlatformType: aws.String(EC2PlatForm),
			},
		},
		{
			name: "NonEC2",
			fields: fields{
				ec2Info: ec2Info{
					InstanceID:       "i-123456789",
					AutoScalingGroup: "test-asg",
				},
				mode: config.ModeOnPrem,
			},
			want: map[string]*string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ResourceStore{
				ec2Info: tt.fields.ec2Info,
				mode:    tt.fields.mode,
			}
			assert.Equalf(t, dereferenceMap(tt.want), dereferenceMap(r.createAttributeMap()), "createAttributeMap()")
		})
	}
}

func TestResourceStore_createServiceKeyAttributes(t *testing.T) {
	tests := []struct {
		name        string
		serviceAttr ServiceAttribute
		want        *cloudwatchlogs.KeyAttributes
	}{
		{
			name:        "NameAndEnvironmentSet",
			serviceAttr: ServiceAttribute{ServiceName: "test-service", Environment: "test-environment"},
			want: &cloudwatchlogs.KeyAttributes{
				Environment: aws.String("test-environment"),
				Name:        aws.String("test-service"),
				Type:        aws.String(Service),
			},
		},
		{
			name:        "OnlyNameSet",
			serviceAttr: ServiceAttribute{ServiceName: "test-service"},
			want: &cloudwatchlogs.KeyAttributes{
				Name: aws.String("test-service"),
				Type: aws.String(Service),
			},
		},
		{
			name:        "OnlyEnvironmentSet",
			serviceAttr: ServiceAttribute{Environment: "test-environment"},
			want: &cloudwatchlogs.KeyAttributes{
				Environment: aws.String("test-environment"),
				Type:        aws.String(Service),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ResourceStore{}
			assert.Equalf(t, tt.want, r.createServiceKeyAttributes(tt.serviceAttr), "createServiceKeyAttributes()")
		})
	}
}

func TestResourceStore_createLogFileRID(t *testing.T) {
	instanceId := "i-abcd1234"
	accountId := "123456789012"
	glob := LogFileGlob("glob")
	group := LogGroupName("group")
	serviceAttr := ServiceAttribute{
		ServiceName:       "test-service",
		ServiceNameSource: ServiceNameSourceUserConfiguration,
		Environment:       "test-environment",
	}
	sp := new(mockServiceProvider)
	sp.On("logFileServiceAttribute", glob, group).Return(serviceAttr)
	rs := ResourceStore{
		mode:             config.ModeEC2,
		ec2Info:          ec2Info{InstanceID: instanceId},
		serviceprovider:  sp,
		metadataprovider: mockMetadataProviderWithAccountId(accountId),
		stsClient:        &mockSTSClient{accountId: accountId},
		nativeCredential: &session.Session{},
	}

	resource := rs.CreateLogFileRID(glob, group)

	expectedResource := cloudwatchlogs.Resource{
		KeyAttributes: &cloudwatchlogs.KeyAttributes{
			Environment: aws.String("test-environment"),
			Name:        aws.String("test-service"),
			Type:        aws.String(Service),
		},
		AttributeMaps: []map[string]*string{
			{
				InstanceIDKey:        aws.String(instanceId),
				ServiceNameSourceKey: aws.String(ServiceNameSourceUserConfiguration),
				PlatformType:         aws.String(EC2PlatForm),
			},
		},
	}
	assert.Equal(t, *expectedResource.KeyAttributes.Environment, *resource.KeyAttributes.Environment)
	assert.Equal(t, *expectedResource.KeyAttributes.Name, *resource.KeyAttributes.Name)
	assert.Equal(t, *expectedResource.KeyAttributes.Type, *resource.KeyAttributes.Type)
	assert.Len(t, resource.AttributeMaps, 1)
	assert.Equal(t, dereferenceMap(expectedResource.AttributeMaps[0]), dereferenceMap(resource.AttributeMaps[0]))
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
		// TODO need tests for when you can't fetch from IMDS or STS (fail closed)
		{
			name: "HappyPath_AccountIDMatches",
			fields: fields{
				metadataprovider: mockMetadataProviderWithAccountId("123456789012"),
				stsClient:        &mockSTSClient{accountId: "123456789012"},
				nativeCredential: &session.Session{},
			},
			want: true,
		},
		{
			name: "HappyPath_AccountIDMismatches",
			fields: fields{
				metadataprovider: mockMetadataProviderWithAccountId("210987654321"),
				stsClient:        &mockSTSClient{accountId: "123456789012"},
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

func TestResourceStore_addServiceAttrEntryForLogFile(t *testing.T) {
	sp := new(mockServiceProvider)
	rs := ResourceStore{serviceprovider: sp}

	key := LogFileGlob("/opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log")
	serviceAttr := ServiceAttribute{
		ServiceName:       "test",
		ServiceNameSource: ServiceNameSourceUserConfiguration,
		Environment:       "ec2:test",
	}
	sp.On("addEntryForLogFile", key, serviceAttr).Return()
	rs.AddServiceAttrEntryForLogFile(key, "test", "ec2:test")

	sp.AssertExpectations(t)
}

func TestResourceStore_addServiceAttrEntryForLogGroup(t *testing.T) {
	sp := new(mockServiceProvider)
	rs := ResourceStore{serviceprovider: sp}

	key := LogGroupName("TestLogGroup")
	serviceAttr := ServiceAttribute{
		ServiceName:       "test",
		ServiceNameSource: ServiceNameSourceInstrumentation,
		Environment:       "ec2:test",
	}
	sp.On("addEntryForLogGroup", key, serviceAttr).Return()
	rs.AddServiceAttrEntryForLogGroup(key, "test", "ec2:test")

	sp.AssertExpectations(t)
}
