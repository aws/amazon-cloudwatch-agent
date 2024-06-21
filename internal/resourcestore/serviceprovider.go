// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resourcestore

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

	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/ec2tagger"
)

const (
	INSTANCE_PROFILE = "instance-profile/"
	SERVICE          = "service"
	APPLICATION      = "application"
	APP              = "app"
	ClientIamRole    = "ClientIamRole"
	ResourceTags     = "ResourceTags"
	jitterMax        = 180
	jitterMin        = 60
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

type serviceprovider struct {
	metadataProvider  ec2metadataprovider.MetadataProvider
	ec2API            ec2iface.EC2API
	ec2Provider       ec2ProviderType
	iamRole           string
	ec2TagServiceName string
	ctx               context.Context

	// logFiles is a variable reserved for communication between OTEL components and LogAgent
	// in order to achieve process correlations where the key is the log file path and the value
	// is the service name
	// Example:
	// "/opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log": "cloudwatch-agent"
	logFiles map[string]ServiceAttribute
}

func (s *serviceprovider) startServiceProvider() {
	err := s.getEC2Client()
	if err != nil {
		go refreshLoop(s.ctx, s.getEC2Client, true)
	}
	go refreshLoop(s.ctx, s.getIAMRole, false)
	go refreshLoop(s.ctx, s.getEC2TagServiceName, false)
}

// ServiceAttribute function gets the relevant service attributes
// service name is retrieved based on the following priority chain
//  1. Incoming telemetry attributes
//  2. CWA config
//  3. Process correlation
//  4. instance tags - The tags attached to the EC2 instance. Only scrape for tag with the following key: service, application, app
//  5. IAM Role - The IAM role name retrieved through IMDS(Instance Metadata Service)
func (s *serviceprovider) ServiceAttribute() ServiceAttribute {
	serviceAttr := ServiceAttribute{}
	if s.ec2TagServiceName != "" {
		serviceAttr.ServiceName = s.ec2TagServiceName
		serviceAttr.ServiceNameSource = ResourceTags
		return serviceAttr
	}
	if s.iamRole != "" {
		serviceAttr.ServiceName = s.iamRole
		serviceAttr.ServiceNameSource = ClientIamRole
		return serviceAttr
	}
	return serviceAttr
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
	region, err := getRegion(s.metadataProvider)
	if err != nil {
		return fmt.Errorf("failed to get EC2 client: %s", err)
	}
	s.ec2API = s.ec2Provider(region)
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

func newServiceProvider(metadataProvider ec2metadataprovider.MetadataProvider, providerType ec2ProviderType) *serviceprovider {
	return &serviceprovider{
		metadataProvider: metadataProvider,
		ec2Provider:      providerType,
		ctx:              context.Background(),
	}
}

func refreshLoop(ctx context.Context, updateFunc func() error, oneTime bool) {
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
		case <-ctx.Done():
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
