// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/ec2tagger"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ec2util"
)

const (
	High_Resolution_Tag_Key      = "aws:StorageResolution"
	Aggregation_Interval_Tag_Key = "aws:AggregationInterval"
)

var ReservedTagKeySet = collections.NewSet[string](High_Resolution_Tag_Key, Aggregation_Interval_Tag_Key, ec2tagger.AttributeVolumeId)

func AddHighResolutionTag(tags interface{}) {
	tagMap := tags.(map[string]interface{})
	tagMap[High_Resolution_Tag_Key] = "true"
}

// FilterReservedKeys out reserved tag keys
func FilterReservedKeys(input any) any {
	result := map[string]any{}
	for k, v := range input.(map[string]interface{}) {
		if !ReservedTagKeySet.Contains(k) {
			result[k] = v
		}
	}
	return result
}

var getEC2TagValueFunc = getEC2TagValue

func GetEC2TagValue(tagKey string) string {
	return getEC2TagValueFunc(tagKey)
}

func getEC2TagValue(tagKey string) string {
	ec2Util := ec2util.GetEC2UtilSingleton()
	if ec2Util.InstanceID == "" || ec2Util.Region == "" {
		return ""
	}

	config := &aws.Config{
		Region:                        aws.String(ec2Util.Region),
		CredentialsChainVerboseErrors: aws.Bool(true),
		LogLevel:                      configaws.SDKLogLevel(),
		Logger:                        configaws.SDKLogger{},
	}

	sess, err := session.NewSession(config)
	if err != nil {
		log.Printf("Failed to create AWS session: %v", err)
		return ""
	}

	ec2Client := ec2.New(sess)

	input := &ec2.DescribeTagsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("resource-type"),
				Values: []*string{aws.String("instance")},
			},
			{
				Name:   aws.String("resource-id"),
				Values: []*string{aws.String(ec2Util.InstanceID)},
			},
			{
				Name:   aws.String("key"),
				Values: []*string{aws.String(tagKey)},
			},
		},
	}

	for {
		result, err := ec2Client.DescribeTags(input)
		if err != nil {
			log.Printf("E! Failed to describe EC2 tag '%s': %v", tagKey, err)
			return ""
		}

		for _, tag := range result.Tags {
			if tag.Key != nil && tag.Value != nil && *tag.Key == tagKey {
				return *tag.Value
			}
		}

		if result.NextToken == nil {
			break
		}
		input.SetNextToken(*result.NextToken)
	}

	return ""
}
