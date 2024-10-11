// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
)

func Test_serviceprovider_startServiceProvider(t *testing.T) {
	tests := []struct {
		name             string
		metadataProvider ec2metadataprovider.MetadataProvider
		wantIAM          string
		wantTag          string
	}{
		{
			name: "HappyPath_AllServiceNames",
			metadataProvider: &mockMetadataProvider{
				InstanceIdentityDocument: &ec2metadata.EC2InstanceIdentityDocument{
					InstanceID: "i-123456789"},
				Tags: map[string]string{"service": "test-service"},
			},
			wantIAM: "TestRole",
			wantTag: "test-service",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			done := make(chan struct{})
			logger, _ := zap.NewDevelopment()
			s := serviceprovider{
				metadataProvider: tt.metadataProvider,
				done:             done,
				logger:           logger,
			}
			go s.startServiceProvider()
			time.Sleep(3 * time.Second)
			close(done)

			assert.Equal(t, tt.wantIAM, s.iamRole)
			assert.Equal(t, tt.wantTag, s.imdsServiceName)
		})
	}
}

func Test_serviceprovider_addEntryForLogFile(t *testing.T) {
	s := &serviceprovider{
		logFiles: make(map[LogFileGlob]ServiceAttribute),
	}
	glob := LogFileGlob("glob")
	serviceAttr := ServiceAttribute{ServiceName: "test-service"}

	s.addEntryForLogFile(glob, serviceAttr)

	actual := s.logFiles[glob]
	assert.Equal(t, serviceAttr, actual)
}

func Test_serviceprovider_addEntryForLogGroup(t *testing.T) {
	s := &serviceprovider{
		logGroups: make(map[LogGroupName]ServiceAttribute),
	}
	group := LogGroupName("group")
	serviceAttr := ServiceAttribute{ServiceName: "test-service"}

	s.addEntryForLogGroup(group, serviceAttr)

	actual := s.logGroups[group]
	assert.Equal(t, serviceAttr, actual)
}

func Test_serviceprovider_mergeServiceAttributes(t *testing.T) {
	onlySvc1 := func() ServiceAttribute {
		return ServiceAttribute{ServiceName: "service1", ServiceNameSource: "source1"}
	}
	onlySvc2 := func() ServiceAttribute {
		return ServiceAttribute{ServiceName: "service2", ServiceNameSource: "source2"}
	}
	onlyEnv1 := func() ServiceAttribute { return ServiceAttribute{Environment: "environment1"} }
	onlyEnv2 := func() ServiceAttribute { return ServiceAttribute{Environment: "environment2"} }
	both2 := func() ServiceAttribute {
		return ServiceAttribute{ServiceName: "service2", ServiceNameSource: "source2", Environment: "environment2"}
	}
	both3 := func() ServiceAttribute {
		return ServiceAttribute{ServiceName: "service3", ServiceNameSource: "source3", Environment: "environment3"}
	}
	empty := func() ServiceAttribute { return ServiceAttribute{} }

	tests := []struct {
		name      string
		providers []serviceAttributeProvider
		want      ServiceAttribute
	}{
		{
			name:      "RespectServicePriority",
			providers: []serviceAttributeProvider{onlySvc1, onlySvc2},
			want:      ServiceAttribute{ServiceName: "service1", ServiceNameSource: "source1"},
		},
		{
			name:      "RespectEnvironmentPriority",
			providers: []serviceAttributeProvider{onlyEnv1, onlyEnv2},
			want:      ServiceAttribute{Environment: "environment1"},
		},
		{
			name:      "CombineServiceAndEnvironment",
			providers: []serviceAttributeProvider{onlySvc1, both2, both3},
			want:      ServiceAttribute{ServiceName: "service1", ServiceNameSource: "source1", Environment: "environment2"},
		},
		{
			name:      "CombineEnvironmentAndService",
			providers: []serviceAttributeProvider{onlyEnv1, both2, both3},
			want:      ServiceAttribute{ServiceName: "service2", ServiceNameSource: "source2", Environment: "environment1"},
		},
		{
			name:      "EmptyList",
			providers: []serviceAttributeProvider{},
			want:      ServiceAttribute{},
		},
		{
			name:      "EmptyProvider",
			providers: []serviceAttributeProvider{empty},
			want:      ServiceAttribute{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, mergeServiceAttributes(tt.providers), "mergeServiceAttributes()")
		})
	}
}

func Test_serviceprovider_serviceAttributeForLogGroup(t *testing.T) {
	s := &serviceprovider{logGroups: map[LogGroupName]ServiceAttribute{"group": {ServiceName: "test-service"}}}
	assert.Equal(t, ServiceAttribute{}, s.serviceAttributeForLogGroup(""))
	assert.Equal(t, ServiceAttribute{}, s.serviceAttributeForLogGroup("othergroup"))
	assert.Equal(t, ServiceAttribute{ServiceName: "test-service"}, s.serviceAttributeForLogGroup("group"))
}

func Test_serviceprovider_serviceAttributeForLogFile(t *testing.T) {
	s := &serviceprovider{logFiles: map[LogFileGlob]ServiceAttribute{"glob": {ServiceName: "test-service"}}}
	assert.Equal(t, ServiceAttribute{}, s.serviceAttributeForLogFile(""))
	assert.Equal(t, ServiceAttribute{}, s.serviceAttributeForLogFile("otherglob"))
	assert.Equal(t, ServiceAttribute{ServiceName: "test-service"}, s.serviceAttributeForLogFile("glob"))
}

func Test_serviceprovider_serviceAttributeFromEc2Tags(t *testing.T) {
	s := &serviceprovider{}
	assert.Equal(t, ServiceAttribute{}, s.serviceAttributeFromImdsTags())

	s = &serviceprovider{imdsServiceName: "test-service"}
	assert.Equal(t, ServiceAttribute{ServiceName: "test-service", ServiceNameSource: ServiceNameSourceResourceTags}, s.serviceAttributeFromImdsTags())
}

func Test_serviceprovider_serviceAttributeFromIamRole(t *testing.T) {
	s := &serviceprovider{}
	assert.Equal(t, ServiceAttribute{}, s.serviceAttributeFromIamRole())

	s = &serviceprovider{iamRole: "test-service"}
	assert.Equal(t, ServiceAttribute{ServiceName: "test-service", ServiceNameSource: ServiceNameSourceClientIamRole}, s.serviceAttributeFromIamRole())
}

func Test_serviceprovider_serviceAttributeFromAsg(t *testing.T) {
	s := &serviceprovider{}
	assert.Equal(t, ServiceAttribute{}, s.serviceAttributeFromAsg())

	s = &serviceprovider{ec2Info: &EC2Info{}}
	assert.Equal(t, ServiceAttribute{}, s.serviceAttributeFromAsg())

	s = &serviceprovider{ec2Info: &EC2Info{AutoScalingGroup: "test-asg"}}
	assert.Equal(t, ServiceAttribute{Environment: "ec2:test-asg"}, s.serviceAttributeFromAsg())
}

func Test_serviceprovider_serviceAttributeFallback(t *testing.T) {
	s := &serviceprovider{}
	assert.Equal(t, ServiceAttribute{ServiceName: ServiceNameUnknown, ServiceNameSource: ServiceNameSourceUnknown}, s.serviceAttributeFallback())

	s = &serviceprovider{mode: config.ModeEC2}
	assert.Equal(t, ServiceAttribute{ServiceName: ServiceNameUnknown, ServiceNameSource: ServiceNameSourceUnknown, Environment: "ec2:default"}, s.serviceAttributeFallback())
}

func Test_serviceprovider_logFileServiceAttribute(t *testing.T) {
	s := &serviceprovider{
		mode:      config.ModeEC2,
		logGroups: make(map[LogGroupName]ServiceAttribute),
		logFiles:  make(map[LogFileGlob]ServiceAttribute),
	}

	// Start with no known source for service attributes, then set values from the bottom of the priority list upward.
	// This way we test the priority order - if we set the highest priority source first (log groups), then we wouldn't
	// be able to test that lower priority sources should be used if necessary.

	assert.Equal(t, ServiceAttribute{ServiceName: ServiceNameUnknown, ServiceNameSource: ServiceNameSourceUnknown, Environment: "ec2:default"}, s.logFileServiceAttribute("glob", "group"))

	s.ec2Info = &EC2Info{AutoScalingGroup: "test-asg"}
	assert.Equal(t, ServiceAttribute{ServiceName: ServiceNameUnknown, ServiceNameSource: ServiceNameSourceUnknown, Environment: "ec2:test-asg"}, s.logFileServiceAttribute("glob", "group"))

	s.iamRole = "test-role"
	assert.Equal(t, ServiceAttribute{ServiceName: "test-role", ServiceNameSource: ServiceNameSourceClientIamRole, Environment: "ec2:test-asg"}, s.logFileServiceAttribute("glob", "group"))

	s.imdsServiceName = "test-service-from-tags"
	assert.Equal(t, ServiceAttribute{ServiceName: "test-service-from-tags", ServiceNameSource: ServiceNameSourceResourceTags, Environment: "ec2:test-asg"}, s.logFileServiceAttribute("glob", "group"))

	s.logFiles["glob"] = ServiceAttribute{ServiceName: "test-service-from-logfile", ServiceNameSource: ServiceNameSourceUserConfiguration}
	assert.Equal(t, ServiceAttribute{ServiceName: "test-service-from-logfile", ServiceNameSource: ServiceNameSourceUserConfiguration, Environment: "ec2:test-asg"}, s.logFileServiceAttribute("glob", "group"))

	s.logGroups["group"] = ServiceAttribute{ServiceName: "test-service-from-loggroup", ServiceNameSource: ServiceNameSourceInstrumentation}
	assert.Equal(t, ServiceAttribute{ServiceName: "test-service-from-loggroup", ServiceNameSource: ServiceNameSourceInstrumentation, Environment: "ec2:test-asg"}, s.logFileServiceAttribute("glob", "group"))
}

func Test_serviceprovider_getServiceNameSource(t *testing.T) {
	s := &serviceprovider{
		mode:      config.ModeEC2,
		logGroups: make(map[LogGroupName]ServiceAttribute),
		logFiles:  make(map[LogFileGlob]ServiceAttribute),
	}

	serviceName, serviceNameSource := s.getServiceNameAndSource()
	assert.Equal(t, ServiceNameUnknown, serviceName)
	assert.Equal(t, ServiceNameSourceUnknown, serviceNameSource)

	s.iamRole = "test-role"
	serviceName, serviceNameSource = s.getServiceNameAndSource()
	assert.Equal(t, s.iamRole, serviceName)
	assert.Equal(t, ServiceNameSourceClientIamRole, serviceNameSource)

	s.imdsServiceName = "test-service-from-tags"
	serviceName, serviceNameSource = s.getServiceNameAndSource()
	assert.Equal(t, s.imdsServiceName, serviceName)
	assert.Equal(t, ServiceNameSourceResourceTags, serviceNameSource)

}

func Test_serviceprovider_getIAMRole(t *testing.T) {
	tests := []struct {
		name             string
		metadataProvider ec2metadataprovider.MetadataProvider
		want             string
	}{
		{
			name:             "Happypath_MockMetadata",
			metadataProvider: &mockMetadataProvider{},
			want:             "TestRole",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := serviceprovider{
				metadataProvider: tt.metadataProvider,
			}
			s.getIAMRole()
			assert.Equal(t, tt.want, s.iamRole)
		})
	}
}

func Test_serviceprovider_getImdsServiceName(t *testing.T) {

	tests := []struct {
		name               string
		metadataProvider   ec2metadataprovider.MetadataProvider
		wantTagServiceName string
	}{
		{
			name:               "HappyPath_ServiceExists",
			metadataProvider:   &mockMetadataProvider{InstanceIdentityDocument: mockedInstanceIdentityDoc, Tags: map[string]string{"service": "test-service"}},
			wantTagServiceName: "test-service",
		},
		{
			name:               "HappyPath_ApplicationExists",
			metadataProvider:   &mockMetadataProvider{InstanceIdentityDocument: mockedInstanceIdentityDoc, Tags: map[string]string{"application": "test-application"}},
			wantTagServiceName: "test-application",
		},
		{
			name:               "HappyPath_AppExists",
			metadataProvider:   &mockMetadataProvider{InstanceIdentityDocument: mockedInstanceIdentityDoc, Tags: map[string]string{"app": "test-app"}},
			wantTagServiceName: "test-app",
		},
		{
			name:               "HappyPath_PreferServiceOverApplication",
			metadataProvider:   &mockMetadataProvider{InstanceIdentityDocument: mockedInstanceIdentityDoc, Tags: map[string]string{"service": "test-service", "application": "test-application"}},
			wantTagServiceName: "test-service",
		},
		{
			name:               "HappyPath_PreferApplicationOverApp",
			metadataProvider:   &mockMetadataProvider{InstanceIdentityDocument: mockedInstanceIdentityDoc, Tags: map[string]string{"application": "test-application", "app": "test-app"}},
			wantTagServiceName: "test-application",
		},
		{
			name:               "HappyPath_PreferServiceOverApplicationAndApp",
			metadataProvider:   &mockMetadataProvider{InstanceIdentityDocument: mockedInstanceIdentityDoc, Tags: map[string]string{"service": "test-service", "application": "test-application", "app": "test-app"}},
			wantTagServiceName: "test-service",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &serviceprovider{
				logger:           zap.NewExample(),
				metadataProvider: tt.metadataProvider,
			}
			s.getImdsServiceName()
			assert.Equal(t, tt.wantTagServiceName, s.imdsServiceName)
		})
	}
}
