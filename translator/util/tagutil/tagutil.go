// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package tagutil

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ec2"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
)

const (
	defaultRetryCount       = 5
	defaultBackoffDuration  = time.Duration(1 * time.Minute)
	eksClusterNameTagPrefix = "kubernetes.io/cluster/"
	autoScalingGroupNameTag = "aws:autoscaling:groupName"
)

var (
	sleeps = []time.Duration{time.Millisecond * 200, time.Millisecond * 400, time.Millisecond * 800, time.Millisecond * 1600, time.Millisecond * 3200}
)

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

type TagsCache struct {
	instanceID string
	tags       sync.Map
	once       sync.Once
}

var tagsCache *TagsCache
var cacheOnce sync.Once

func getTagsCache(instanceID string) *TagsCache {
	cacheOnce.Do(func() {
		tagsCache = &TagsCache{
			instanceID: instanceID,
		}
	})
	return tagsCache
}

func (tc *TagsCache) loadAllTags() {
	tc.once.Do(func() {
		ec2API := ec2APIProvider()
		if ec2API == nil {
			log.Printf("W! loadAllTags: EC2 API client not available for instance %s", tc.instanceID)
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
					Values: aws.StringSlice([]string{tc.instanceID}),
				},
			},
		}

		result, err := ec2API.DescribeTagsWithContext(ctx, input)
		if err != nil {
			log.Printf("W! loadAllTags: Failed to describe tags for instance %s: %v", tc.instanceID, err)
			return
		}

		for _, tag := range result.Tags {
			tc.tags.Store(*tag.Key, *tag.Value)
		}

		log.Printf("D! loadAllTags: Loaded %d tags for instance %s", len(result.Tags), tc.instanceID)
	})
}

func SetEC2APIProviderForTesting(provider EC2APIProvider) {
	ec2APIProvider = provider
}

func ResetEC2APIProvider() {
	ec2APIProvider = defaultEC2APIProvider
}

func ResetTagsCache() {
	cacheOnce = sync.Once{}
	tagsCache = nil
}

// sleep some back off time before retries.
func backoffSleep(i int) {
	backoffDuration := getBackoffDuration(i)
	log.Printf("D! It is the %v time, going to sleep %v before retrying.", i, backoffDuration)
	time.Sleep(backoffDuration)
}

func getBackoffDuration(i int) time.Duration {
	backoffDuration := defaultBackoffDuration
	if i >= 0 && i < len(sleeps) {
		backoffDuration = sleeps[i]
	}
	return backoffDuration
}

// GetAutoScalingGroupName gets the AutoScaling Group name for an instance
func GetAutoScalingGroupName(instanceID string) string {
	if instanceID == "" {
		return ""
	}

	tc := getTagsCache(instanceID)
	tc.loadAllTags()

	var asgName string
	tc.tags.Range(func(key, value interface{}) bool {
		if key.(string) == autoScalingGroupNameTag {
			asgName = value.(string)
			return false
		}
		return true
	})

	return asgName
}

// GetEKSClusterName gets the EKS cluster name for an instance
// Uses retry and backoff logic if the tag doesn't exist
func GetEKSClusterName(instanceID string) string {
	if instanceID == "" {
		return ""
	}

	// Use retry logic for EKS cluster name
	for i := 0; i <= defaultRetryCount; i++ {
		tc := getTagsCache(instanceID)
		tc.loadAllTags()

		clusterName := checkForEKSClusterInCache(tc)
		if clusterName != "" {
			return clusterName
		}

		// If no cluster name found and we have more attempts, wait and retry
		if i < defaultRetryCount {
			log.Printf("D! EKS cluster name not found, will retry. Attempt %d/%d", i+1, defaultRetryCount+1)
			backoffSleep(i)
			// Reset the tags cache to force a fresh fetch on next attempt
			ResetTagsCache()
		}
	}

	return ""
}

// checkForEKSClusterInCache checks if EKS cluster tags exist in the cache
func checkForEKSClusterInCache(tc *TagsCache) string {
	var clusterName string
	tc.tags.Range(func(key, value interface{}) bool {
		keyStr := key.(string)
		valueStr := value.(string)

		// Look for kubernetes.io/cluster/<cluster-name> tags with value "owned"
		if strings.HasPrefix(keyStr, eksClusterNameTagPrefix) && valueStr == "owned" {
			clusterName = keyStr[len(eksClusterNameTagPrefix):]
			return false
		}
		return true
	})
	return clusterName
}
