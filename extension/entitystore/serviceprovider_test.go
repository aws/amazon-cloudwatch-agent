// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

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
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
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
	assert.Equal(t, ServiceAttribute{}, s.serviceAttributeFromEc2Tags())

	s = &serviceprovider{ec2TagServiceName: "test-service"}
	assert.Equal(t, ServiceAttribute{ServiceName: "test-service", ServiceNameSource: ServiceNameSourceResourceTags}, s.serviceAttributeFromEc2Tags())
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

	s = &serviceprovider{ec2Info: &ec2Info{}}
	assert.Equal(t, ServiceAttribute{}, s.serviceAttributeFromAsg())

	s = &serviceprovider{ec2Info: &ec2Info{AutoScalingGroup: "test-asg"}}
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

	s.ec2Info = &ec2Info{AutoScalingGroup: "test-asg"}
	assert.Equal(t, ServiceAttribute{ServiceName: ServiceNameUnknown, ServiceNameSource: ServiceNameSourceUnknown, Environment: "ec2:test-asg"}, s.logFileServiceAttribute("glob", "group"))

	s.iamRole = "test-role"
	assert.Equal(t, ServiceAttribute{ServiceName: "test-role", ServiceNameSource: ServiceNameSourceClientIamRole, Environment: "ec2:test-asg"}, s.logFileServiceAttribute("glob", "group"))

	s.ec2TagServiceName = "test-service-from-tags"
	assert.Equal(t, ServiceAttribute{ServiceName: "test-service-from-tags", ServiceNameSource: ServiceNameSourceResourceTags, Environment: "ec2:test-asg"}, s.logFileServiceAttribute("glob", "group"))

	s.logFiles["glob"] = ServiceAttribute{ServiceName: "test-service-from-logfile", ServiceNameSource: ServiceNameSourceUserConfiguration}
	assert.Equal(t, ServiceAttribute{ServiceName: "test-service-from-logfile", ServiceNameSource: ServiceNameSourceUserConfiguration, Environment: "ec2:test-asg"}, s.logFileServiceAttribute("glob", "group"))

	s.logGroups["group"] = ServiceAttribute{ServiceName: "test-service-from-loggroup", ServiceNameSource: ServiceNameSourceInstrumentation}
	assert.Equal(t, ServiceAttribute{ServiceName: "test-service-from-loggroup", ServiceNameSource: ServiceNameSourceInstrumentation, Environment: "ec2:test-asg"}, s.logFileServiceAttribute("glob", "group"))
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
