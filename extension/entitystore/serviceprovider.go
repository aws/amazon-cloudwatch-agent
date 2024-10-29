// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws/arn"
	"go.uber.org/zap"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/ec2tagger"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
)

const (
	INSTANCE_PROFILE = "instance-profile/"
	SERVICE          = "service"
	APPLICATION      = "application"
	APP              = "app"

	// Matches the default value from OTel
	// https://opentelemetry.io/docs/languages/sdk-configuration/general/#otel_service_name
	ServiceNameUnknown = "unknown_service"

	ServiceNameSourceClientIamRole     = "ClientIamRole"
	ServiceNameSourceInstrumentation   = "Instrumentation"
	ServiceNameSourceResourceTags      = "ResourceTags"
	ServiceNameSourceUnknown           = "Unknown"
	ServiceNameSourceUserConfiguration = "UserConfiguration"
	ServiceNameSourceK8sWorkload       = "K8sWorkload"

	describeTagsJitterMax = 3600
	describeTagsJitterMin = 3000
	defaultJitterMin      = 60
	defaultJitterMax      = 180
	maxRetry              = 3
)

var (
	//priorityMap is ranking in how we prioritize which IMDS tag determines the service name
	priorityMap = []string{SERVICE, APPLICATION, APP}
)

type ServiceAttribute struct {
	ServiceName       string
	ServiceNameSource string
	Environment       string
}

type LogGroupName string
type LogFileGlob string

type serviceprovider struct {
	mode             string
	ec2Info          *EC2Info
	metadataProvider ec2metadataprovider.MetadataProvider
	iamRole          string
	imdsServiceName  string
	region           string
	done             chan struct{}
	logger           *zap.Logger
	mutex            sync.RWMutex
	// logFiles stores the service attributes that were configured for log files in CloudWatch Agent configuration.
	// Example:
	// "/opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log": {ServiceName: "cloudwatch-agent"}
	logFiles map[LogFileGlob]ServiceAttribute

	// logGroups stores the associations between log groups and service attributes that were observed from incoming
	// telemetry.  Example:
	// "MyLogGroup": {ServiceName: "MyInstrumentedService"}
	logGroups map[LogGroupName]ServiceAttribute
}

func (s *serviceprovider) startServiceProvider() {
	unlimitedRetryer := NewRetryer(false, true, defaultJitterMin, defaultJitterMax, ec2tagger.BackoffSleepArray, infRetry, s.done, s.logger)
	limitedRetryer := NewRetryer(false, false, describeTagsJitterMin, describeTagsJitterMax, ec2tagger.ThrottleBackOffArray, maxRetry, s.done, s.logger)
	go unlimitedRetryer.refreshLoop(s.scrapeIAMRole)
	go limitedRetryer.refreshLoop(s.scrapeImdsServiceName)
}

func (s *serviceprovider) GetIAMRole() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.iamRole
}

func (s *serviceprovider) GetIMDSServiceName() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.imdsServiceName
}

// addEntryForLogFile adds an association between a log file glob and a service attribute, as configured in the
// CloudWatch Agent config.
func (s *serviceprovider) addEntryForLogFile(logFileGlob LogFileGlob, serviceAttr ServiceAttribute) {
	s.logFiles[logFileGlob] = serviceAttr
}

// addEntryForLogGroup adds an association between a log group name and a service attribute, as observed from incoming
// telemetry received by CloudWatch Agent.
func (s *serviceprovider) addEntryForLogGroup(logGroupName LogGroupName, serviceAttr ServiceAttribute) {
	s.logGroups[logGroupName] = serviceAttr
}

type serviceAttributeProvider func() ServiceAttribute

// mergeServiceAttributes takes in a list of functions that create ServiceAttributes, in descending priority order
// (highest priority first), and proceeds down the list until we have obtained both a ServiceName and an
// EnvironmentName.
func mergeServiceAttributes(providers []serviceAttributeProvider) ServiceAttribute {
	ret := ServiceAttribute{}

	for _, provider := range providers {
		serviceAttr := provider()

		if ret.ServiceName == "" {
			ret.ServiceName = serviceAttr.ServiceName
			ret.ServiceNameSource = serviceAttr.ServiceNameSource
		}
		if ret.Environment == "" {
			ret.Environment = serviceAttr.Environment
		}

		if ret.ServiceName != "" && ret.Environment != "" {
			return ret
		}
	}

	return ret
}

// logFileServiceAttribute function gets the relevant service attributes
// service name is retrieved based on the following priority chain
//  1. Incoming telemetry attributes
//  2. CWA config
//  3. instance tags - The tags attached to the EC2 instance. Only scrape for tag with the following key: service, application, app
//  4. IAM Role - The IAM role name retrieved through IMDS(Instance Metadata Service)
func (s *serviceprovider) logFileServiceAttribute(logFile LogFileGlob, logGroup LogGroupName) ServiceAttribute {
	return mergeServiceAttributes([]serviceAttributeProvider{
		func() ServiceAttribute { return s.serviceAttributeForLogGroup(logGroup) },
		func() ServiceAttribute { return s.serviceAttributeForLogFile(logFile) },
		s.serviceAttributeFromImdsTags,
		s.serviceAttributeFromIamRole,
		s.serviceAttributeFromAsg,
		s.serviceAttributeFallback,
	})
}

func (s *serviceprovider) getServiceNameAndSource() (string, string) {
	sa := mergeServiceAttributes([]serviceAttributeProvider{
		s.serviceAttributeFromImdsTags,
		s.serviceAttributeFromIamRole,
		s.serviceAttributeFallback,
	})
	return sa.ServiceName, sa.ServiceNameSource
}

func (s *serviceprovider) serviceAttributeForLogGroup(logGroup LogGroupName) ServiceAttribute {
	if logGroup == "" {
		return ServiceAttribute{}
	}

	return s.logGroups[logGroup]
}

func (s *serviceprovider) serviceAttributeForLogFile(logFile LogFileGlob) ServiceAttribute {
	if logFile == "" {
		return ServiceAttribute{}
	}

	return s.logFiles[logFile]
}

func (s *serviceprovider) serviceAttributeFromImdsTags() ServiceAttribute {
	if s.GetIMDSServiceName() == "" {
		return ServiceAttribute{}
	}

	return ServiceAttribute{
		ServiceName:       s.GetIMDSServiceName(),
		ServiceNameSource: ServiceNameSourceResourceTags,
	}
}

func (s *serviceprovider) serviceAttributeFromIamRole() ServiceAttribute {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	if s.GetIAMRole() == "" {
		return ServiceAttribute{}
	}

	return ServiceAttribute{
		ServiceName:       s.GetIAMRole(),
		ServiceNameSource: ServiceNameSourceClientIamRole,
	}
}

func (s *serviceprovider) serviceAttributeFromAsg() ServiceAttribute {
	if s.ec2Info == nil || s.ec2Info.GetAutoScalingGroup() == "" {
		return ServiceAttribute{}
	}

	return ServiceAttribute{
		Environment: "ec2:" + s.ec2Info.GetAutoScalingGroup(),
	}
}

func (s *serviceprovider) serviceAttributeFallback() ServiceAttribute {
	attr := ServiceAttribute{
		ServiceName:       ServiceNameUnknown,
		ServiceNameSource: ServiceNameSourceUnknown,
	}
	if s.mode == config.ModeEC2 {
		attr.Environment = "ec2:default"
	}

	return attr
}

func (s *serviceprovider) scrapeIAMRole() error {
	iamRole, err := s.metadataProvider.InstanceProfileIAMRole()
	if err != nil {
		return err
	}
	iamRoleArn, err := arn.Parse(iamRole)
	if err != nil {
		return err
	}
	iamRoleResource := iamRoleArn.Resource
	if strings.HasPrefix(iamRoleResource, INSTANCE_PROFILE) {
		roleName := strings.TrimPrefix(iamRoleResource, INSTANCE_PROFILE)
		s.mutex.Lock()
		s.iamRole = roleName
		s.mutex.Unlock()
	} else {
		return fmt.Errorf("IAM Role resource does not follow the expected pattern. Should be instance-profile/<role_name>")
	}
	return nil
}
func (s *serviceprovider) scrapeImdsServiceName() error {
	tags, err := s.metadataProvider.InstanceTags(context.Background())
	if err != nil {
		s.logger.Debug("Failed to get tags through metadata provider", zap.Error(err))
		return err
	}
	// This will check whether the tags contains SERVICE, APPLICATION, APP, in that order.
	for _, value := range priorityMap {
		if strings.Contains(tags, value) {
			serviceName, err := s.metadataProvider.InstanceTagValue(context.Background(), value)
			if err != nil {
				continue
			} else {
				s.mutex.Lock()
				s.imdsServiceName = serviceName
				s.mutex.Unlock()
			}
			break
		}
	}
	if s.GetIMDSServiceName() == "" {
		s.logger.Debug("Service name not found through IMDS")
	}
	return nil
}

func newServiceProvider(mode string, region string, ec2Info *EC2Info, metadataProvider ec2metadataprovider.MetadataProvider, providerType ec2ProviderType, ec2Credential *configaws.CredentialConfig, done chan struct{}, logger *zap.Logger) serviceProviderInterface {
	return &serviceprovider{
		mode:             mode,
		region:           region,
		ec2Info:          ec2Info,
		metadataProvider: metadataProvider,
		done:             done,
		logger:           logger,
		logFiles:         make(map[LogFileGlob]ServiceAttribute),
		logGroups:        make(map[LogGroupName]ServiceAttribute),
	}
}
