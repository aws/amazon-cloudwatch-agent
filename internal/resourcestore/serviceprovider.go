// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resourcestore

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"

	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
)

const (
	INSTANCE_PROFILE = "instance-profile/"
	SERVICE          = "service"
	APPLICATION      = "application"
	APP              = "app"
	ClientIamRole    = "ClientIamRole"
	ResourceTags     = "ResourceTags"
)

var (
	priorityMap = map[string]int{
		SERVICE:     2,
		APPLICATION: 1,
		APP:         0,
	}
)

type ServiceAttribute struct {
	serviceName       string
	serviceNameSource string
	environment       string
}

type serviceprovider struct {
	metadataProvider  ec2metadataprovider.MetadataProvider
	ec2API            ec2iface.EC2API
	ec2Provider       ec2ProviderType
	iamRole           string
	ec2TagServiceName string
}

func (s *serviceprovider) startServiceProvider() {
	go func() {
		err := s.getIAMRole()
		if err != nil {
			log.Println("D! serviceprovider failed to get service name through IAM role in service provider: ", err)
		}
	}()
	region, err := getRegion(s.metadataProvider)
	if err != nil {
		log.Println("D! serviceprovider failed to get region: ", err)
	}
	go func() {
		s.ec2API = s.ec2Provider(region)
		err := s.getEC2TagServiceName()
		if err != nil {
			log.Println("D! serviceprovider failed to get service name through EC2 tags in service provider: ", err)
		}
	}()
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
		serviceAttr.serviceName = s.ec2TagServiceName
		serviceAttr.serviceNameSource = ResourceTags
		return serviceAttr
	}
	if s.iamRole != "" {
		serviceAttr.serviceName = s.iamRole
		serviceAttr.serviceNameSource = ClientIamRole
		return serviceAttr
	}
	return serviceAttr
}

func (s *serviceprovider) getIAMRole() error {
	iamRole, err := s.metadataProvider.InstanceProfileIAMRole()
	if err != nil {
		log.Println("D! resourceMap: Unable to retrieve EC2 Metadata. This feature must only be used on an EC2 instance.")
		return err
	}
	iamRoleArn, err := arn.Parse(iamRole)
	if err != nil {
		log.Println("D! resourceMap: Unable to parse IAM Role Arn. " + err.Error())
	}
	iamRoleResource := iamRoleArn.Resource
	if strings.HasPrefix(iamRoleResource, INSTANCE_PROFILE) {
		roleName := strings.TrimPrefix(iamRoleResource, INSTANCE_PROFILE)
		s.iamRole = roleName
	} else {
		log.Println("D! resourceMap: IAM Role resource does not follow the expected pattern. Should be instance-profile/<role_name>")
	}
	return nil
}

func (s *serviceprovider) getEC2TagServiceName() error {
	serviceTagFilters, err := s.getEC2TagFilters()
	if err != nil {
		return err
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
	}
}
