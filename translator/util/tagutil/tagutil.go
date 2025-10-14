// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package tagutil

import (
	"log"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ec2util"
)

const (
	defaultRetryCount          = 5
	defaultBackoffDuration     = time.Duration(1 * time.Minute)
	EKSClusterNameTagKeyPrefix = "kubernetes.io/cluster/"
	autoScalingGroupNameTag    = "aws:autoscaling:groupName"
	HighResolutionTagKey       = "aws:StorageResolution"
	AggregationIntervalTagKey  = "aws:AggregationInterval"
)

var (
	sleeps = []time.Duration{time.Millisecond * 200, time.Millisecond * 400, time.Millisecond * 800, time.Millisecond * 1600, time.Millisecond * 3200}
)

type EC2TagsClient interface {
	DescribeTags(input *ec2.DescribeTagsInput) (*ec2.DescribeTagsOutput, error)
}

type EC2APIProvider func() EC2TagsClient

var ec2APIProvider EC2APIProvider

// TagsCache holds the cached tags for an instance
type TagsCache struct {
	instanceID string
	tags       map[string]string
	once       sync.Once
}

var tagsCache *TagsCache
var cacheOnce sync.Once

func getTagsCache(instanceID string) *TagsCache {
	cacheOnce.Do(func() {
		tagsCache = &TagsCache{
			instanceID: instanceID,
			tags:       make(map[string]string),
		}
	})
	return tagsCache
}

// sleep some back off time before retries.
func backoffSleep(i int) {
	backoffDuration := getBackoffDuration(i)
	log.Printf("W! It is the %v time, going to sleep %v before retrying.", i, backoffDuration)
	time.Sleep(backoffDuration)
}

func getBackoffDuration(i int) time.Duration {
	backoffDuration := defaultBackoffDuration
	if i >= 0 && i < len(sleeps) {
		backoffDuration = sleeps[i]
	}
	return backoffDuration
}

// encapsulate the retry logic in this separate method.
func callFuncWithRetries(fn func(input *ec2.DescribeTagsInput) (*ec2.DescribeTagsOutput, error), input *ec2.DescribeTagsInput, errorMsg string) (*ec2.DescribeTagsOutput, error) {
	for i := 0; i <= defaultRetryCount; i++ {
		result, err := fn(input)
		if err == nil {
			return result, nil
		}
		log.Printf("%s Will retry the request: %s", errorMsg, err.Error())
		backoffSleep(i)
	}
	return nil, nil
}

// loadAllTags loads all tags for the instance using retry logic
func (tc *TagsCache) loadAllTags() {
	tc.once.Do(func() {
		region := ec2util.GetEC2UtilSingleton().Region
		if region == "" && ec2APIProvider == nil {
			log.Printf("W! loadAllTags: No region available and no test provider for instance %s", tc.instanceID)
			return
		}

		tagFilters := []*ec2.Filter{
			{
				Name:   aws.String("resource-type"),
				Values: aws.StringSlice([]string{"instance"}),
			},
			{
				Name:   aws.String("resource-id"),
				Values: aws.StringSlice([]string{tc.instanceID}),
			},
		}

		config := &aws.Config{
			Region:                        aws.String(region),
			CredentialsChainVerboseErrors: aws.Bool(true),
			LogLevel:                      configaws.SDKLogLevel(),
			Logger:                        configaws.SDKLogger{},
		}

		input := &ec2.DescribeTagsInput{
			Filters: tagFilters,
		}

		var ec2Client EC2TagsClient
		if ec2APIProvider != nil {
			ec2Client = ec2APIProvider()
		} else {
			if region == "" {
				log.Printf("W! loadAllTags: No region available for instance %s", tc.instanceID)
				return
			}
			ses, err := session.NewSession(config)
			if err != nil {
				log.Printf("E! loadAllTags: Failed to create session for instance %s: %v", tc.instanceID, err)
				return
			}
			ec2Client = ec2.New(ses)
		}

		totalTags := 0
		for {
			result, err := callFuncWithRetries(ec2Client.DescribeTags, input, "Describe EC2 Tag Fail.")
			if err != nil {
				log.Printf("E! loadAllTags: DescribeTags failed for instance %s: %v", tc.instanceID, err)
				return
			}

			// Store all tags from this page
			for _, tag := range result.Tags {
				tc.tags[*tag.Key] = *tag.Value
			}
			totalTags += len(result.Tags)

			// Check if there are more pages
			if result.NextToken == nil {
				break
			}

			// Set the next token for the next page
			input.SetNextToken(*result.NextToken)
		}

		log.Printf("D! loadAllTags: Loaded %d tags for instance %s", totalTags, tc.instanceID)
	})
}

// GetAutoScalingGroupName gets the AutoScaling Group name for an instance
func GetAutoScalingGroupName(instanceID string) string {
	if instanceID == "" {
		return ""
	}

	tc := getTagsCache(instanceID)
	tc.loadAllTags()

	return tc.tags[autoScalingGroupNameTag]
}

// GetEKSClusterName gets the EKS cluster name for an instance
func GetEKSClusterName(instanceID string) string {
	if instanceID == "" {
		return ""
	}

	tc := getTagsCache(instanceID)
	tc.loadAllTags()

	// Look for kubernetes.io/cluster/<cluster-name> tags with value "owned"
	for key, value := range tc.tags {
		if strings.HasPrefix(key, EKSClusterNameTagKeyPrefix) && value == "owned" {
			return key[len(EKSClusterNameTagKeyPrefix):]
		}
	}

	return ""
}

// AddHighResolutionTag adds the high resolution tag to the provided tags map
func AddHighResolutionTag(tags interface{}) {
	tagMap := tags.(map[string]interface{})
	tagMap[HighResolutionTagKey] = "true"
}

// FilterReservedKeys filters out reserved tag keys from the input
func FilterReservedKeys(input any) any {
	result := map[string]any{}
	for k, v := range input.(map[string]interface{}) {
		if k != HighResolutionTagKey && k != AggregationIntervalTagKey {
			result[k] = v
		}
	}
	return result
}

func SetEC2APIProviderForTesting(provider EC2APIProvider) {
	ec2APIProvider = provider
}

func ResetEC2APIProvider() {
	ec2APIProvider = nil
}

func ResetTagsCache() {
	cacheOnce = sync.Once{}
	tagsCache = nil
}
