// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resourcestore

import (
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/aws/arn"

	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
)

const (
	INSTANCE_PROFILE = "instance-profile/"
)

type serviceprovider struct {
	iamRole string
}

func (s *serviceprovider) startServiceProvider(metadataProvider ec2metadataprovider.MetadataProvider) error {
	err := s.getIAMRole(metadataProvider)
	if err != nil {
		log.Println("D! Failed to get IAM role through service provider")
		return err
	}
	return nil
}

// ServiceName function gets the relevant service name based
// on the following priority chain
//  1. Incoming telemetry attributes
//  2. CWA config
//  3. Process correlation
//  4. instance tags
//  5. IAM Role - The IAM role name retrieved through IMDS(Instance Metadata Service)
func (s *serviceprovider) ServiceName() string {
	return s.iamRole
}

func (s *serviceprovider) getIAMRole(metadataProvider ec2metadataprovider.MetadataProvider) error {
	iamRole, err := metadataProvider.InstanceProfileIAMRole()
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

func newServiceProvider() *serviceprovider {
	return &serviceprovider{}
}
