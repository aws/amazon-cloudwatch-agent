// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

import (
	"context"
	"strings"
	"sync"

	"go.uber.org/zap"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/ec2tagger"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
)

const (
	SERVICE     = "service"
	APPLICATION = "application"
	APP         = "app"

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
	defaultJitterMin      = 480
	defaultJitterMax      = 600
	maxRetry              = 3
)

var (
	//serviceProviderPriorities is ranking in how we prioritize which IMDS tag determines the service name
	serviceProviderPriorities = []string{SERVICE, APPLICATION, APP}
)

type ServiceAttribute struct {
	ServiceName       string
	ServiceNameSource string
	Environment       string
}

type LogGroupName string
type LogFileGlob string

type autoscalinggroup struct {
	name string
	once sync.Once
}

type serviceprovider struct {
	mode             string
	ec2Info          *EC2Info
	metadataProvider ec2metadataprovider.MetadataProvider
	iamRole          string
	imdsServiceName  string
	autoScalingGroup autoscalinggroup
	region           string
	done             chan struct{}
	logger           *zap.Logger
	mutex            sync.RWMutex
	logMutex         sync.RWMutex
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
	if s.metadataProvider == nil {
		return
	}
	unlimitedRetryer := NewRetryer(false, true, defaultJitterMin, defaultJitterMax, ec2tagger.BackoffSleepArray, infRetry, s.done, s.logger)
	unlimitedRetryerUntilSuccess := NewRetryer(true, true, describeTagsJitterMin, describeTagsJitterMax, ec2tagger.BackoffSleepArray, infRetry, s.done, s.logger)
	go unlimitedRetryer.refreshLoop(s.scrapeIAMRole)
	go unlimitedRetryerUntilSuccess.refreshLoop(s.scrapeImdsServiceNameAndASG)
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

func (s *serviceprovider) getAutoScalingGroup() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.autoScalingGroup.name
}

func (s *serviceprovider) setAutoScalingGroup(asg string) {
	s.autoScalingGroup.once.Do(func() {
		s.mutex.Lock()
		defer s.mutex.Unlock()

		if asgLength := len(asg); asgLength > autoScalingGroupSizeMax {
			s.logger.Warn("AutoScalingGroup length exceeds characters limit and will be ignored", zap.Int("length", asgLength), zap.Int("character limit", autoScalingGroupSizeMax))
			s.autoScalingGroup.name = ""
		} else {
			s.autoScalingGroup.name = asg
		}
	})
}

// addEntryForLogFile adds an association between a log file glob and a service attribute, as configured in the
// CloudWatch Agent config.
func (s *serviceprovider) addEntryForLogFile(logFileGlob LogFileGlob, serviceAttr ServiceAttribute) {
	s.logMutex.Lock()
	defer s.logMutex.Unlock()
	if s.logFiles == nil {
		s.logFiles = make(map[LogFileGlob]ServiceAttribute)
	}
	s.logFiles[logFileGlob] = serviceAttr
}

// addEntryForLogGroup adds an association between a log group name and a service attribute, as observed from incoming
// telemetry received by CloudWatch Agent.
func (s *serviceprovider) addEntryForLogGroup(logGroupName LogGroupName, serviceAttr ServiceAttribute) {
	s.logMutex.Lock()
	defer s.logMutex.Unlock()
	if s.logGroups == nil {
		s.logGroups = make(map[LogGroupName]ServiceAttribute)
	}
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
	if logGroup == "" || s.logGroups == nil {
		return ServiceAttribute{}
	}
	s.logMutex.RLock()
	defer s.logMutex.RUnlock()
	return s.logGroups[logGroup]
}

func (s *serviceprovider) serviceAttributeForLogFile(logFile LogFileGlob) ServiceAttribute {
	if logFile == "" || s.logFiles == nil {
		return ServiceAttribute{}
	}
	s.logMutex.RLock()
	defer s.logMutex.RUnlock()
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
	if s.GetIAMRole() == "" {
		return ServiceAttribute{}
	}

	return ServiceAttribute{
		ServiceName:       s.GetIAMRole(),
		ServiceNameSource: ServiceNameSourceClientIamRole,
	}
}

func (s *serviceprovider) serviceAttributeFromAsg() ServiceAttribute {
	if s.getAutoScalingGroup() == "" {
		return ServiceAttribute{}
	}

	return ServiceAttribute{
		Environment: "ec2:" + s.getAutoScalingGroup(),
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
	iamRole, err := s.metadataProvider.ClientIAMRole(context.Background())
	if err != nil {
		return err
	}
	s.mutex.Lock()
	s.iamRole = iamRole
	s.mutex.Unlock()
	return nil
}
func (s *serviceprovider) scrapeImdsServiceNameAndASG() error {
	tagKeys, err := s.metadataProvider.InstanceTags(context.Background())
	if err != nil {
		s.logger.Debug("Failed to get service name from instance tags. This is likely because instance tag is not enabled for IMDS but will not affect agent functionality.")
		return err
	}

	// This will check whether the tags contains SERVICE, APPLICATION, APP, in that order (case insensitive)
	lowerTagKeys := toLowerKeyMap(tagKeys)
	for _, potentialServiceProviderKey := range serviceProviderPriorities {
		if originalCaseKey, exists := lowerTagKeys[potentialServiceProviderKey]; exists {
			serviceName, err := s.metadataProvider.InstanceTagValue(context.Background(), originalCaseKey)
			if err != nil {
				continue
			}
			s.mutex.Lock()
			s.imdsServiceName = serviceName
			s.mutex.Unlock()
			break
		}
	}
	// case sensitive
	if originalCaseKey := lowerTagKeys[strings.ToLower(ec2tagger.Ec2InstanceTagKeyASG)]; originalCaseKey == ec2tagger.Ec2InstanceTagKeyASG {
		asg, err := s.metadataProvider.InstanceTagValue(context.Background(), ec2tagger.Ec2InstanceTagKeyASG)
		if err == nil && asg != "" {
			s.logger.Debug("AutoScalingGroup retrieved through IMDS")
			s.setAutoScalingGroup(asg)
		}
	}

	if s.GetIMDSServiceName() == "" {
		s.logger.Debug("Service name not found through IMDS")
	}
	if s.getAutoScalingGroup() == "" {
		s.logger.Debug("AutoScalingGroup name not found through IMDS")
	}
	return nil
}

func toLowerKeyMap(values []string) map[string]string {
	set := make(map[string]string, len(values))
	for _, v := range values {
		set[strings.ToLower(v)] = v
	}
	return set
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
