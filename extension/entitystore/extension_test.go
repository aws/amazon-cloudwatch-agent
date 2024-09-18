// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
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
	InstanceTagError         bool
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
	if m.InstanceTagError {
		return "", errors.New("an error occurred for instance tag retrieval")
	}
	return m.Tags, nil
}

func (m *mockMetadataProvider) InstanceTagValue(ctx context.Context, tagKey string) (string, error) {
	return m.TagValue, nil
}

func TestEntityStore_EC2Info(t *testing.T) {
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
			e := &EntityStore{
				ec2Info: tt.ec2InfoInput,
			}
			if got := e.EC2Info(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EC2Info() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEntityStore_Mode(t *testing.T) {
	tests := []struct {
		name      string
		modeInput string
		want      string
	}{
		{name: "happypath", modeInput: "EC2", want: "EC2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &EntityStore{
				mode: tt.modeInput,
			}
			if got := e.Mode(); got != tt.want {
				t.Errorf("Mode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEntityStore_KubernetesMode(t *testing.T) {
	tests := []struct {
		name         string
		k8sModeInput string
		want         string
	}{
		{name: "modeEKS", k8sModeInput: config.ModeEKS, want: config.ModeEKS},
		{name: "modeK8sEc2", k8sModeInput: config.ModeK8sEC2, want: config.ModeK8sEC2},
		{name: "modeK8sOnPrem", k8sModeInput: config.ModeK8sOnPrem, want: config.ModeK8sOnPrem},
		{name: "modeNotSet", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &EntityStore{}
			e.kubernetesMode = tt.k8sModeInput
			if got := e.KubernetesMode(); got != tt.want {
				t.Errorf("Kubernetes Mode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEntityStore_createAttributeMaps(t *testing.T) {
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
			e := &EntityStore{
				ec2Info: tt.fields.ec2Info,
				mode:    tt.fields.mode,
			}
			assert.Equalf(t, dereferenceMap(tt.want), dereferenceMap(e.createAttributeMap()), "createAttributeMap()")
		})
	}
}

func TestEntityStore_createServiceKeyAttributes(t *testing.T) {
	tests := []struct {
		name        string
		serviceAttr ServiceAttribute
		want        map[string]*string
	}{
		{
			name:        "NameAndEnvironmentSet",
			serviceAttr: ServiceAttribute{ServiceName: "test-service", Environment: "test-environment"},
			want: map[string]*string{
				Environment: aws.String("test-environment"),
				Name:        aws.String("test-service"),
				Type:        aws.String(Service),
			},
		},
		{
			name:        "OnlyNameSet",
			serviceAttr: ServiceAttribute{ServiceName: "test-service"},
			want: map[string]*string{
				Name: aws.String("test-service"),
				Type: aws.String(Service),
			},
		},
		{
			name:        "OnlyEnvironmentSet",
			serviceAttr: ServiceAttribute{Environment: "test-environment"},
			want: map[string]*string{
				Environment: aws.String("test-environment"),
				Type:        aws.String(Service),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &EntityStore{}
			assert.Equalf(t, dereferenceMap(tt.want), dereferenceMap(e.createServiceKeyAttributes(tt.serviceAttr)), "createServiceKeyAttributes()")
		})
	}
}

func TestEntityStore_createLogFileRID(t *testing.T) {
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
	e := EntityStore{
		mode:             config.ModeEC2,
		ec2Info:          ec2Info{InstanceID: instanceId},
		serviceprovider:  sp,
		metadataprovider: mockMetadataProviderWithAccountId(accountId),
		stsClient:        &mockSTSClient{accountId: accountId},
		nativeCredential: &session.Session{},
	}

	entity := e.CreateLogFileEntity(glob, group)

	expectedEntity := cloudwatchlogs.Entity{
		KeyAttributes: map[string]*string{
			Environment: aws.String("test-environment"),
			Name:        aws.String("test-service"),
			Type:        aws.String(Service),
		},
		Attributes: map[string]*string{
			InstanceIDKey:        aws.String(instanceId),
			ServiceNameSourceKey: aws.String(ServiceNameSourceUserConfiguration),
			PlatformType:         aws.String(EC2PlatForm),
		},
	}
	assert.Equal(t, dereferenceMap(expectedEntity.KeyAttributes), dereferenceMap(entity.KeyAttributes))
	assert.Equal(t, dereferenceMap(expectedEntity.Attributes), dereferenceMap(entity.Attributes))
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

func TestEntityStore_addServiceAttrEntryForLogFile(t *testing.T) {
	sp := new(mockServiceProvider)
	e := EntityStore{serviceprovider: sp}

	key := LogFileGlob("/opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log")
	serviceAttr := ServiceAttribute{
		ServiceName:       "test",
		ServiceNameSource: ServiceNameSourceUserConfiguration,
		Environment:       "ec2:test",
	}
	sp.On("addEntryForLogFile", key, serviceAttr).Return()
	e.AddServiceAttrEntryForLogFile(key, "test", "ec2:test")

	sp.AssertExpectations(t)
}

func TestEntityStore_addServiceAttrEntryForLogGroup(t *testing.T) {
	sp := new(mockServiceProvider)
	e := EntityStore{serviceprovider: sp}

	key := LogGroupName("TestLogGroup")
	serviceAttr := ServiceAttribute{
		ServiceName:       "test",
		ServiceNameSource: ServiceNameSourceInstrumentation,
		Environment:       "ec2:test",
	}
	sp.On("addEntryForLogGroup", key, serviceAttr).Return()
	e.AddServiceAttrEntryForLogGroup(key, "test", "ec2:test")

	sp.AssertExpectations(t)
}

func TestEntityStore_AddAndGetPodServiceEnvironmentMapping(t *testing.T) {
	logger, _ := zap.NewProduction()
	tests := []struct {
		name string
		want map[string]ServiceEnvironment
		eks  *eksInfo
	}{
		{
			name: "HappyPath",
			want: map[string]ServiceEnvironment{
				"pod1": {
					ServiceName: "service1",
					Environment: "env1",
				},
			},
			eks: newEKSInfo(logger),
		},
		{
			name: "Empty EKS Info",
			want: map[string]ServiceEnvironment{},
			eks:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := EntityStore{eksInfo: tt.eks}
			e.AddPodServiceEnvironmentMapping("pod1", "service1", "env1")
			assert.Equal(t, tt.want, e.GetPodServiceEnvironmentMapping())
		})
	}
}
