// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package tagutil

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws/v2"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ec2util"
)

const (
	defaultRetryCount          = 5
	defaultBackoffDuration     = 1 * time.Minute
	EKSClusterNameTagKeyPrefix = "kubernetes.io/cluster/"
	autoScalingGroupNameTag    = "aws:autoscaling:groupName"
	HighResolutionTagKey       = "aws:StorageResolution"
	AggregationIntervalTagKey  = "aws:AggregationInterval"
)

var (
	sleeps = []time.Duration{time.Millisecond * 200, time.Millisecond * 400, time.Millisecond * 800, time.Millisecond * 1600, time.Millisecond * 3200}
)

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
func callFuncWithRetries(fn func() (*ec2.DescribeTagsOutput, error), errorMsg string) (*ec2.DescribeTagsOutput, error) {
	for i := 0; i <= defaultRetryCount; i++ {
		result, err := fn()
		if err == nil {
			return result, nil
		}
		log.Printf("%s Will retry the request: %s", errorMsg, err.Error())
		backoffSleep(i)
	}
	return nil, nil
}

var ec2ClientFactory = createDescribeTagsClient

func createDescribeTagsClient(ctx context.Context) (ec2.DescribeTagsAPIClient, error) {
	region := ec2util.GetEC2UtilSingleton().Region
	if region == "" {
		return nil, nil
	}

	cfg := configaws.CredentialsConfig{
		Region: region,
	}
	awsCfg, err := cfg.LoadConfig(ctx)
	if err != nil {
		return nil, err
	}

	return ec2.NewFromConfig(awsCfg), nil
}

// loadAllTags loads all tags for the instance using retry logic
func (tc *TagsCache) loadAllTags(ctx context.Context) {
	tc.once.Do(func() {
		ec2Client, err := ec2ClientFactory(ctx)
		if err != nil {
			log.Printf("E! loadAllTags: Failed to create EC2 client: %v", err)
			return
		}
		if ec2Client == nil {
			log.Printf("W! loadAllTags: No region available for instance %s", tc.instanceID)
			return
		}

		tagFilters := []types.Filter{
			{
				Name:   aws.String("resource-type"),
				Values: []string{"instance"},
			},
			{
				Name:   aws.String("resource-id"),
				Values: []string{tc.instanceID},
			},
		}

		input := &ec2.DescribeTagsInput{
			Filters: tagFilters,
		}

		totalTags := 0
		paginator := ec2.NewDescribeTagsPaginator(ec2Client, input)
		for paginator.HasMorePages() {
			result, err := callFuncWithRetries(func() (*ec2.DescribeTagsOutput, error) {
				return paginator.NextPage(ctx)
			}, "Describe EC2 Tag Fail.")

			if err != nil {
				log.Printf("E! loadAllTags: DescribeTags failed: %v", err)
				return
			}

			// Store all tags from this page
			for _, tag := range result.Tags {
				tc.tags[*tag.Key] = *tag.Value
			}
			totalTags += len(result.Tags)
		}

		log.Printf("D! loadAllTags: Loaded %d tags", totalTags)
	})
}

// GetAutoScalingGroupName gets the AutoScaling Group name for an instance
func GetAutoScalingGroupName(ctx context.Context, instanceID string) string {
	if instanceID == "" {
		return ""
	}

	tc := getTagsCache(instanceID)
	tc.loadAllTags(ctx)

	return tc.tags[autoScalingGroupNameTag]
}

// GetEKSClusterName gets the EKS cluster name for an instance
func GetEKSClusterName(ctx context.Context, instanceID string) string {
	if instanceID == "" {
		return ""
	}

	tc := getTagsCache(instanceID)
	tc.loadAllTags(ctx)

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

// Test functions
func SetEC2APIProviderForTesting(provider func() ec2.DescribeTagsAPIClient) {
	ec2ClientFactory = func(context.Context) (ec2.DescribeTagsAPIClient, error) {
		return provider(), nil
	}
}

func ResetEC2APIProvider() {
	ec2ClientFactory = createDescribeTagsClient
}

func ResetTagsCache() {
	cacheOnce = sync.Once{}
	tagsCache = nil
}
