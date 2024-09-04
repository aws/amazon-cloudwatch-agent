// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"

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

	jitterMax = 180
	jitterMin = 60
)

var (
	priorityMap = map[string]int{
		SERVICE:     2,
		APPLICATION: 1,
		APP:         0,
	}
)

type ServiceAttribute struct {
	ServiceName       string
	ServiceNameSource string
	Environment       string
}

type LogGroupName string
type LogFileGlob string

type serviceprovider struct {
	mode              string
	ec2Info           *ec2Info
	metadataProvider  ec2metadataprovider.MetadataProvider
	ec2API            ec2iface.EC2API
	ec2Provider       ec2ProviderType
	ec2Credential     *configaws.CredentialConfig
	iamRole           string
	ec2TagServiceName string
	region            string
	done              chan struct{}

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
	err := s.getEC2Client()
	if err != nil {
		go refreshLoop(s.done, s.getEC2Client, true)
	}
	go refreshLoop(s.done, s.getIAMRole, false)
	go refreshLoop(s.done, s.getEC2TagServiceName, false)
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
		s.serviceAttributeFromEc2Tags,
		s.serviceAttributeFromIamRole,
		s.serviceAttributeFromAsg,
		s.serviceAttributeFallback,
	})
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

func (s *serviceprovider) serviceAttributeFromEc2Tags() ServiceAttribute {
	if s.ec2TagServiceName == "" {
		return ServiceAttribute{}
	}

	return ServiceAttribute{
		ServiceName:       s.ec2TagServiceName,
		ServiceNameSource: ServiceNameSourceResourceTags,
	}
}

func (s *serviceprovider) serviceAttributeFromIamRole() ServiceAttribute {
	if s.iamRole == "" {
		return ServiceAttribute{}
	}

	return ServiceAttribute{
		ServiceName:       s.iamRole,
		ServiceNameSource: ServiceNameSourceClientIamRole,
	}
}

func (s *serviceprovider) serviceAttributeFromAsg() ServiceAttribute {
	if s.ec2Info == nil || s.ec2Info.AutoScalingGroup == "" {
		return ServiceAttribute{}
	}

	return ServiceAttribute{
		Environment: "ec2:" + s.ec2Info.AutoScalingGroup,
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

func (s *serviceprovider) getIAMRole() error {
	iamRole, err := s.metadataProvider.InstanceProfileIAMRole()
	if err != nil {
		return fmt.Errorf("failed to get instance profile role: %s", err)
	}
	iamRoleArn, err := arn.Parse(iamRole)
	if err != nil {
		return fmt.Errorf("failed to parse IAM Role Arn: %s", err)
	}
	iamRoleResource := iamRoleArn.Resource
	if strings.HasPrefix(iamRoleResource, INSTANCE_PROFILE) {
		roleName := strings.TrimPrefix(iamRoleResource, INSTANCE_PROFILE)
		s.iamRole = roleName
	} else {
		return fmt.Errorf("IAM Role resource does not follow the expected pattern. Should be instance-profile/<role_name>")
	}
	return nil
}

func (s *serviceprovider) getEC2TagServiceName() error {
	if s.ec2API == nil {
		return fmt.Errorf("can't get EC2 tag since client is not set up yet ")
	}
	serviceTagFilters, err := s.getEC2TagFilters()
	if err != nil {
		return fmt.Errorf("failed to get service name from EC2 tag: %s", err)
	}
	currentTagPriority := -1
	for {
		input := &ec2.DescribeTagsInput{
			Filters: serviceTagFilters,
		}
		result, err := s.ec2API.DescribeTags(input)
		if err != nil {
			continue
		}
		for _, tag := range result.Tags {
			key := *tag.Key
			value := *tag.Value
			if priority, found := priorityMap[key]; found {
				if priority > currentTagPriority {
					s.ec2TagServiceName = value
					currentTagPriority = priority
				}
			}
		}
		if result.NextToken == nil {
			break
		}
		input.SetNextToken(*result.NextToken)
	}
	return nil
}

func (s *serviceprovider) getEC2Client() error {
	if s.ec2API != nil {
		return nil
	}
	s.ec2API = s.ec2Provider(s.region, s.ec2Credential)
	return nil
}

func (s *serviceprovider) getEC2TagFilters() ([]*ec2.Filter, error) {
	instanceDocument, err := s.metadataProvider.Get(context.Background())
	if err != nil {
		return nil, errors.New("failed to get instance document")
	}
	instanceID := instanceDocument.InstanceID
	tagFilters := []*ec2.Filter{
		{
			Name:   aws.String("resource-type"),
			Values: aws.StringSlice([]string{"instance"}),
		},
		{
			Name:   aws.String("resource-id"),
			Values: aws.StringSlice([]string{instanceID}),
		},
		{
			Name:   aws.String("key"),
			Values: aws.StringSlice([]string{SERVICE, APPLICATION, APP}),
		},
	}
	return tagFilters, nil
}

func newServiceProvider(mode string, region string, ec2Info *ec2Info, metadataProvider ec2metadataprovider.MetadataProvider, providerType ec2ProviderType, ec2Credential *configaws.CredentialConfig, done chan struct{}) serviceProviderInterface {
	return &serviceprovider{
		mode:             mode,
		region:           region,
		ec2Info:          ec2Info,
		metadataProvider: metadataProvider,
		ec2Provider:      providerType,
		ec2Credential:    ec2Credential,
		done:             done,
		logFiles:         make(map[LogFileGlob]ServiceAttribute),
		logGroups:        make(map[LogGroupName]ServiceAttribute),
	}
}

func refreshLoop(done chan struct{}, updateFunc func() error, oneTime bool) {
	// Offset retry by 1 so we can start with 1 minute wait time
	// instead of immediately retrying
	retry := 1
	for {
		err := updateFunc()
		if err == nil && oneTime {
			return
		}

		waitDuration := calculateWaitTime(retry, err)
		wait := time.NewTimer(waitDuration)
		select {
		case <-done:
			log.Printf("D! serviceprovider: Shutting down now")
			wait.Stop()
			return
		case <-wait.C:
		}

		if retry > 1 {
			log.Printf("D! serviceprovider: attribute retrieval retry count: %d", retry-1)
		}

		if err != nil {
			retry++
			log.Printf("D! serviceprovider: there was an error when retrieving service attribute. Reason: %s", err)
		} else {
			retry = 1
		}

	}
}

// calculateWaitTime returns different time based on whether if
// a function call was returned with error. If returned with error,
// follow exponential backoff wait time, otherwise, refresh with jitter
func calculateWaitTime(retry int, err error) time.Duration {
	var waitDuration time.Duration
	if err == nil {
		return time.Duration(rand.Intn(jitterMax-jitterMin)+jitterMin) * time.Second
	}
	if retry < len(ec2tagger.BackoffSleepArray) {
		waitDuration = ec2tagger.BackoffSleepArray[retry]
	} else {
		waitDuration = ec2tagger.BackoffSleepArray[len(ec2tagger.BackoffSleepArray)-1]
	}
	return waitDuration
}
