// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

import (
	"bytes"
	"context"
	"errors"
	"log"
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/jellydator/ttlcache/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/exp/maps"

	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsentity/entityattributes"
	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
)

type mockServiceProvider struct {
	mock.Mock
}

// This helper function creates a test logger
// so that it can send the log messages into a
// temporary buffer for pattern matching
func CreateTestLogger(buf *bytes.Buffer) *zap.Logger {
	writer := zapcore.AddSync(buf)

	// Create a custom zapcore.Core that writes to the buffer
	encoder := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	core := zapcore.NewCore(encoder, writer, zapcore.DebugLevel)
	logger := zap.New(core)
	return logger
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

func (s *mockServiceProvider) getServiceNameAndSource() (string, string) {
	return "test-service-name", "UserConfiguration"
}

func (s *mockServiceProvider) getAutoScalingGroup() string {
	args := s.Called()
	return args.Get(0).(string)
}

type mockMetadataProvider struct {
	InstanceIdentityDocument *ec2metadata.EC2InstanceIdentityDocument
	Tags                     map[string]string
	InstanceTagError         bool
}

func mockMetadataProviderFunc() ec2metadataprovider.MetadataProvider {
	return &mockMetadataProvider{
		Tags: map[string]string{
			"aws:autoscaling:groupName": "ASG-1",
		},
		InstanceIdentityDocument: &ec2metadata.EC2InstanceIdentityDocument{
			InstanceID: "i-123456789",
		},
	}
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

func (m *mockMetadataProvider) InstanceTags(_ context.Context) ([]string, error) {
	if m.InstanceTagError {
		return nil, errors.New("an error occurred for instance tag retrieval")
	}
	return maps.Keys(m.Tags), nil
}

func (m *mockMetadataProvider) ClientIAMRole(ctx context.Context) (string, error) {
	return "TestRole", nil
}

func (m *mockMetadataProvider) InstanceTagValue(ctx context.Context, tagKey string) (string, error) {
	tag, ok := m.Tags[tagKey]
	if !ok {
		return "", errors.New("tag not found")
	}
	return tag, nil
}

func TestEntityStore_EC2Info(t *testing.T) {
	tests := []struct {
		name         string
		ec2InfoInput EC2Info
		want         EC2Info
	}{
		{
			name: "happypath",
			ec2InfoInput: EC2Info{
				InstanceID: "i-1234567890",
			},
			want: EC2Info{
				InstanceID: "i-1234567890",
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
		ec2Info  EC2Info
		mode     string
		emptyASG bool
	}
	tests := []struct {
		name   string
		fields fields
		want   map[string]*string
	}{
		{
			name: "HappyPath",
			fields: fields{
				ec2Info: EC2Info{
					InstanceID: "i-123456789",
				},
				mode: config.ModeEC2,
			},
			want: map[string]*string{
				ASGKey:        aws.String("ASG-1"),
				InstanceIDKey: aws.String("i-123456789"),
				PlatformType:  aws.String(EC2PlatForm),
			},
		},
		{
			name: "HappyPath_AsgMissing",
			fields: fields{
				ec2Info: EC2Info{
					InstanceID: "i-123456789",
				},
				mode:     config.ModeEC2,
				emptyASG: true,
			},
			want: map[string]*string{
				InstanceIDKey: aws.String("i-123456789"),
				PlatformType:  aws.String(EC2PlatForm),
			},
		},
		{
			name: "HappyPath_InstanceIdAndAsgMissing",
			fields: fields{
				mode:     config.ModeEC2,
				emptyASG: true,
			},
			want: map[string]*string{
				PlatformType: aws.String(EC2PlatForm),
			},
		},
		{
			name: "NonEC2",
			fields: fields{
				ec2Info: EC2Info{
					InstanceID: "i-123456789",
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
			sp := new(mockServiceProvider)
			if tt.fields.emptyASG {
				sp.On("getAutoScalingGroup").Return("")
			} else {
				sp.On("getAutoScalingGroup").Return("ASG-1")
			}
			e.serviceprovider = sp
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
				entityattributes.DeploymentEnvironment: aws.String("test-environment"),
				entityattributes.ServiceName:           aws.String("test-service"),
				entityattributes.EntityType:            aws.String(Service),
			},
		},
		{
			name:        "OnlyNameSet",
			serviceAttr: ServiceAttribute{ServiceName: "test-service"},
			want: map[string]*string{
				entityattributes.ServiceName: aws.String("test-service"),
				entityattributes.EntityType:  aws.String(Service),
			},
		},
		{
			name:        "OnlyEnvironmentSet",
			serviceAttr: ServiceAttribute{Environment: "test-environment"},
			want: map[string]*string{
				entityattributes.DeploymentEnvironment: aws.String("test-environment"),
				entityattributes.EntityType:            aws.String(Service),
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
	sp.On("getAutoScalingGroup").Return("ASG-1")
	e := EntityStore{
		mode:             config.ModeEC2,
		ec2Info:          EC2Info{InstanceID: instanceId, AccountID: accountId},
		serviceprovider:  sp,
		nativeCredential: &session.Session{},
	}

	entity := e.CreateLogFileEntity(glob, group)

	expectedEntity := cloudwatchlogs.Entity{
		KeyAttributes: map[string]*string{
			entityattributes.DeploymentEnvironment: aws.String("test-environment"),
			entityattributes.ServiceName:           aws.String("test-service"),
			entityattributes.EntityType:            aws.String(Service),
			entityattributes.AwsAccountId:          aws.String(accountId),
		},
		Attributes: map[string]*string{
			InstanceIDKey:                     aws.String(instanceId),
			ServiceNameSourceKey:              aws.String(ServiceNameSourceUserConfiguration),
			PlatformType:                      aws.String(EC2PlatForm),
			entityattributes.AutoscalingGroup: aws.String("ASG-1"),
		},
	}
	assert.Equal(t, dereferenceMap(expectedEntity.KeyAttributes), dereferenceMap(entity.KeyAttributes))
	assert.Equal(t, dereferenceMap(expectedEntity.Attributes), dereferenceMap(entity.Attributes))
}

func TestEntityStore_createLogFileRID_ServiceProviderIsEmpty(t *testing.T) {
	instanceId := "i-abcd1234"
	glob := LogFileGlob("glob")
	group := LogGroupName("group")
	e := EntityStore{
		mode:             config.ModeEC2,
		ec2Info:          EC2Info{InstanceID: instanceId},
		nativeCredential: &session.Session{},
	}

	entity := e.CreateLogFileEntity(glob, group)

	assert.Nil(t, entity)
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
		want *ttlcache.Cache[string, ServiceEnvironment]
		eks  *eksInfo
	}{
		{
			name: "HappyPath",
			want: setupTTLCacheForTesting(map[string]ServiceEnvironment{
				"pod1": {
					ServiceName:       "service1",
					Environment:       "env1",
					ServiceNameSource: ServiceNameSourceK8sWorkload,
				},
			}, ttlDuration),
			eks: newEKSInfo(logger),
		},
		{
			name: "Empty EKS Info",
			want: setupTTLCacheForTesting(map[string]ServiceEnvironment{}, ttlDuration),
			eks:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := EntityStore{eksInfo: tt.eks}
			e.AddPodServiceEnvironmentMapping("pod1", "service1", "env1", ServiceNameSourceK8sWorkload)
			for pod, se := range tt.want.Items() {
				assert.Equal(t, se.Value(), e.GetPodServiceEnvironmentMapping().Get(pod).Value())
			}
			assert.Equal(t, tt.want.Len(), e.GetPodServiceEnvironmentMapping().Len())
		})
	}
}

func TestEntityStore_ClearTerminatedPodsFromServiceMap(t *testing.T) {
	logger, _ := zap.NewProduction()
	tests := []struct {
		name            string
		podToServiceMap *ttlcache.Cache[string, ServiceEnvironment]
		want            *ttlcache.Cache[string, ServiceEnvironment]
		eks             *eksInfo
	}{
		{
			name: "HappyPath_NoClear",
			podToServiceMap: setupTTLCacheForTesting(map[string]ServiceEnvironment{
				"pod1": {
					ServiceName: "service1",
					Environment: "env1",
				},
			}, ttlDuration),
			want: setupTTLCacheForTesting(map[string]ServiceEnvironment{
				"pod1": {
					ServiceName: "service1",
					Environment: "env1",
				},
			}, ttlDuration),
			eks: newEKSInfo(logger),
		},
		{
			name: "HappyPath_Clear",
			podToServiceMap: setupTTLCacheForTesting(map[string]ServiceEnvironment{
				"pod1": {
					ServiceName: "service1",
					Environment: "env1",
				},
			}, time.Nanosecond),
			want: setupTTLCacheForTesting(map[string]ServiceEnvironment{}, time.Nanosecond),
			eks:  newEKSInfo(logger),
		},
		{
			name: "Empty EKS Info",
			want: setupTTLCacheForTesting(map[string]ServiceEnvironment{}, ttlDuration),
			eks:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := EntityStore{eksInfo: tt.eks}
			if tt.eks != nil {
				e.eksInfo.podToServiceEnvMap = tt.podToServiceMap
				go e.eksInfo.podToServiceEnvMap.Start()
			}
			//sleep for 1 second to allow the cache to update
			time.Sleep(1 * time.Second)
			for pod, se := range tt.want.Items() {
				assert.Equal(t, se.Value(), e.GetPodServiceEnvironmentMapping().Get(pod).Value())
			}
			if tt.eks != nil {
				e.eksInfo.podToServiceEnvMap.Stop()
			}
			assert.Equal(t, tt.want.Len(), e.GetPodServiceEnvironmentMapping().Len())
		})
	}
}

func TestEntityStore_StartPodToServiceEnvironmentMappingTtlCache(t *testing.T) {
	e := EntityStore{eksInfo: newEKSInfo(zap.NewExample())}
	e.done = make(chan struct{})
	e.eksInfo.podToServiceEnvMap = setupTTLCacheForTesting(map[string]ServiceEnvironment{}, time.Microsecond)

	go e.StartPodToServiceEnvironmentMappingTtlCache()
	assert.Equal(t, 0, e.GetPodServiceEnvironmentMapping().Len())
	e.AddPodServiceEnvironmentMapping("pod", "service", "env", "Instrumentation")
	assert.Equal(t, 1, e.GetPodServiceEnvironmentMapping().Len())

	// sleep for 1 second to allow the cache to update
	time.Sleep(time.Second)

	//cache should be cleared
	assert.Equal(t, 0, e.GetPodServiceEnvironmentMapping().Len())

}

func TestEntityStore_StopPodToServiceEnvironmentMappingTtlCache(t *testing.T) {
	e := EntityStore{eksInfo: newEKSInfo(zap.NewExample())}
	e.done = make(chan struct{})
	e.eksInfo.podToServiceEnvMap = setupTTLCacheForTesting(map[string]ServiceEnvironment{}, time.Second)
	e.logger = zap.NewNop()

	go e.StartPodToServiceEnvironmentMappingTtlCache()
	assert.Equal(t, 0, e.GetPodServiceEnvironmentMapping().Len())
	e.AddPodServiceEnvironmentMapping("pod", "service", "env", "Instrumentation")
	assert.Equal(t, 1, e.GetPodServiceEnvironmentMapping().Len())

	time.Sleep(time.Millisecond)
	assert.NoError(t, e.Shutdown(nil))
	//cache should be cleared
	time.Sleep(time.Second)
	assert.Equal(t, 1, e.GetPodServiceEnvironmentMapping().Len())
}

func TestEntityStore_GetMetricServiceNameSource(t *testing.T) {
	instanceId := "i-abcd1234"
	accountId := "123456789012"
	sp := new(mockServiceProvider)
	e := EntityStore{
		mode:             config.ModeEC2,
		ec2Info:          EC2Info{InstanceID: instanceId},
		serviceprovider:  sp,
		metadataprovider: mockMetadataProviderWithAccountId(accountId),
		nativeCredential: &session.Session{},
	}

	serviceName, serviceNameSource := e.GetMetricServiceNameAndSource()

	assert.Equal(t, "test-service-name", serviceName)
	assert.Equal(t, "UserConfiguration", serviceNameSource)
}

func TestEntityStore_GetMetricServiceNameSource_ServiceProviderEmpty(t *testing.T) {
	instanceId := "i-abcd1234"
	accountId := "123456789012"
	e := EntityStore{
		mode:             config.ModeEC2,
		ec2Info:          EC2Info{InstanceID: instanceId},
		metadataprovider: mockMetadataProviderWithAccountId(accountId),
		nativeCredential: &session.Session{},
	}

	serviceName, serviceNameSource := e.GetMetricServiceNameAndSource()

	assert.Equal(t, "", serviceName)
	assert.Equal(t, "", serviceNameSource)
}

func TestEntityStore_LogMessageDoesNotIncludeResourceInfo(t *testing.T) {
	type args struct {
		metadataProvider ec2metadataprovider.MetadataProvider
		mode             string
		kubernetesMode   string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "AutoScalingGroupWithInstanceTagsEC2",
			args: args{
				mode: config.ModeEC2,
			},
		},
		{
			name: "AutoScalingGroupWithInstanceTagsEKS",
			args: args{
				kubernetesMode: config.ModeEKS,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a buffer to capture the logger output
			var buf bytes.Buffer

			logger := CreateTestLogger(&buf)
			done := make(chan struct{})
			config := &Config{
				Mode:           tt.args.mode,
				KubernetesMode: tt.args.kubernetesMode,
			}
			getMetaDataProvider = mockMetadataProviderFunc
			es := &EntityStore{
				logger:           logger,
				done:             done,
				metadataprovider: tt.args.metadataProvider,
				config:           config,
			}
			go es.Start(nil, nil)
			time.Sleep(2 * time.Second)

			logOutput := buf.String()
			log.Println(logOutput)
			assertIfNonEmpty(t, logOutput, es.ec2Info.GetInstanceID())
			assertIfNonEmpty(t, logOutput, es.GetAutoScalingGroup())
			assertIfNonEmpty(t, logOutput, es.ec2Info.GetAccountID())
			assert.True(t, es.ready.Load(), "EntityStore should be ready")
		})
	}
}

func TestEntityStore_ServiceProviderInDifferentEnv(t *testing.T) {
	type args struct {
		mode           string
		kubernetesMode string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "EC2inEKS",
			args: args{
				mode:           config.ModeEC2,
				kubernetesMode: config.ModeEKS,
			},
		},
		{
			name: "EC2Only",
			args: args{
				mode: config.ModeEC2,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			esConfig := &Config{
				Mode:           tt.args.mode,
				KubernetesMode: tt.args.kubernetesMode,
			}
			getMetaDataProvider = mockMetadataProviderFunc
			e := EntityStore{
				logger: zap.NewNop(),
				config: esConfig,
			}
			e.Start(context.TODO(), nil)
			time.Sleep(3 * time.Second)

			name, source := e.serviceprovider.getServiceNameAndSource()
			if tt.args.mode == config.ModeEC2 && tt.args.kubernetesMode != "" {
				assert.Equal(t, name, ServiceNameUnknown)
				assert.Equal(t, source, ServiceNameSourceUnknown)
			} else if tt.args.mode == config.ModeEC2 && tt.args.kubernetesMode == "" {
				assert.Equal(t, name, "TestRole")
				assert.Equal(t, source, ServiceNameSourceClientIamRole)
			}

		})
	}

}

func assertIfNonEmpty(t *testing.T, message string, pattern string) {
	if pattern != "" {
		assert.NotContains(t, message, pattern)
	}
}
