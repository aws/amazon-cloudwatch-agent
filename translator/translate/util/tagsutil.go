// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"context"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ec2"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/ec2tagger"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
)

// EC2TagsClient interface for EC2 tags operations
type EC2TagsClient interface {
	DescribeTagsWithContext(ctx aws.Context, input *ec2.DescribeTagsInput, opts ...request.Option) (*ec2.DescribeTagsOutput, error)
}

// EC2APIProvider provides EC2 API client
type EC2APIProvider func() EC2TagsClient

// Default EC2 API provider
var defaultEC2APIProvider = func() EC2TagsClient {
	ec2CredentialConfig := &configaws.CredentialConfig{
		Region: agent.Global_Config.Region,
	}
	return ec2.New(
		ec2CredentialConfig.Credentials(),
		&aws.Config{
			LogLevel: configaws.SDKLogLevel(),
			Logger:   configaws.SDKLogger{},
		})
}

var ec2APIProvider EC2APIProvider = defaultEC2APIProvider

// getEC2TagValue fetches a specific tag value from EC2 tags for a given instance
func getEC2TagValue(instanceID, tagKey string) string {
	ec2API := ec2APIProvider()
	if ec2API == nil {
		log.Printf("W! getEC2TagValue: EC2 API client not available for tag %s", tagKey)
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	input := &ec2.DescribeTagsInput{
		Filters: []*ec2.Filter{
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
				Values: aws.StringSlice([]string{tagKey}),
			},
		},
	}

	result, err := ec2API.DescribeTagsWithContext(ctx, input)
	if err != nil {
		log.Printf("W! getEC2TagValue: Failed to describe tags for instance %s, tag %s: %v", instanceID, tagKey, err)
		return ""
	}

	for _, tag := range result.Tags {
		if *tag.Key == tagKey {
			return *tag.Value
		}
	}

	return ""
}

// getAutoScalingGroupName fetches the AutoScalingGroupName from EC2 tags
func getAutoScalingGroupName(instanceID string) string {
	return getEC2TagValue(instanceID, ec2tagger.Ec2InstanceTagKeyASG)
}

// SetEC2APIProviderForTesting allows setting a custom EC2 API provider for testing
func SetEC2APIProviderForTesting(provider EC2APIProvider) {
	ec2APIProvider = provider
}

// ResetEC2APIProvider resets the EC2 API provider to default (for testing purposes)
func ResetEC2APIProvider() {
	ec2APIProvider = defaultEC2APIProvider
}

// GetEC2TagValue is exported for getting any EC2 tag value
func GetEC2TagValue(instanceID, tagKey string) string {
	return getEC2TagValue(instanceID, tagKey)
}

// GetAutoScalingGroupName is exported for testing
func GetAutoScalingGroupName(instanceID string) string {
	return getAutoScalingGroupName(instanceID)
}
