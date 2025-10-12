// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ec2"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/ec2tagger"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
)

const (
	High_Resolution_Tag_Key      = "aws:StorageResolution"
	Aggregation_Interval_Tag_Key = "aws:AggregationInterval"
)

var ReservedTagKeySet = collections.NewSet[string](High_Resolution_Tag_Key, Aggregation_Interval_Tag_Key, ec2tagger.AttributeVolumeId)

type EC2TagsClient interface {
	DescribeTagsWithContext(ctx aws.Context, input *ec2.DescribeTagsInput, opts ...request.Option) (*ec2.DescribeTagsOutput, error)
}

type EC2APIProvider func() EC2TagsClient

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

type TagsSingleton struct {
	instanceID string
	tags       map[string]string
	mu         sync.RWMutex
	once       sync.Once
}

var tagsSingleton *TagsSingleton
var singletonOnce sync.Once

func getTagsSingleton(instanceID string) *TagsSingleton {
	singletonOnce.Do(func() {
		tagsSingleton = &TagsSingleton{
			instanceID: instanceID,
			tags:       make(map[string]string),
		}
	})
	return tagsSingleton
}

func (ts *TagsSingleton) loadAllTags() {
	ts.once.Do(func() {
		ec2API := ec2APIProvider()
		if ec2API == nil {
			log.Printf("W! loadAllTags: EC2 API client not available for instance %s", ts.instanceID)
			return
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
					Values: aws.StringSlice([]string{ts.instanceID}),
				},
			},
		}

		result, err := ec2API.DescribeTagsWithContext(ctx, input)
		if err != nil {
			log.Printf("W! loadAllTags: Failed to describe tags for instance %s: %v", ts.instanceID, err)
			return
		}

		ts.mu.Lock()
		defer ts.mu.Unlock()

		for _, tag := range result.Tags {
			ts.tags[*tag.Key] = *tag.Value
		}

		log.Printf("D! loadAllTags: Loaded %d tags for instance %s", len(ts.tags), ts.instanceID)
	})
}

func (ts *TagsSingleton) getTag(tagKey string) string {
	ts.loadAllTags()

	ts.mu.RLock()
	defer ts.mu.RUnlock()

	return ts.tags[tagKey]
}

func getAutoScalingGroupName(instanceID string) string {
	ts := getTagsSingleton(instanceID)
	return ts.getTag(ec2tagger.Ec2InstanceTagKeyASG)
}

func SetEC2APIProviderForTesting(provider EC2APIProvider) {
	ec2APIProvider = provider
}

func ResetEC2APIProvider() {
	ec2APIProvider = defaultEC2APIProvider
}

func ResetTagsSingleton() {
	singletonOnce = sync.Once{}
	tagsSingleton = nil
}

func GetEC2TagValue(instanceID, tagKey string) string {
	ts := getTagsSingleton(instanceID)
	return ts.getTag(tagKey)
}

func GetAutoScalingGroupName(instanceID string) string {
	return getAutoScalingGroupName(instanceID)
}

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
