// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

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
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ec2util"
)

type EC2TagsClient interface {
	DescribeTagsWithContext(ctx aws.Context, input *ec2.DescribeTagsInput, opts ...request.Option) (*ec2.DescribeTagsOutput, error)
}

type EC2APIProvider func() EC2TagsClient

var defaultEKSEC2APIProvider = func() EC2TagsClient {
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

var eksEC2APIProvider EC2APIProvider = defaultEKSEC2APIProvider

type EKSTagsCache struct {
	instanceID string
	tags       map[string]string
	mu         sync.RWMutex
	once       sync.Once
}

var eksTagsCache *EKSTagsCache
var eksCacheOnce sync.Once

func getEKSTagsCache(instanceID string) *EKSTagsCache {
	eksCacheOnce.Do(func() {
		eksTagsCache = &EKSTagsCache{
			instanceID: instanceID,
			tags:       make(map[string]string),
		}
	})
	return eksTagsCache
}

func (etc *EKSTagsCache) loadAllTags() {
	etc.once.Do(func() {
		ec2API := eksEC2APIProvider()
		if ec2API == nil {
			log.Printf("W! loadAllTags: EC2 API client not available for instance %s", etc.instanceID)
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
					Values: aws.StringSlice([]string{etc.instanceID}),
				},
			},
		}

		result, err := ec2API.DescribeTagsWithContext(ctx, input)
		if err != nil {
			log.Printf("W! loadAllTags: Failed to describe tags for instance %s: %v", etc.instanceID, err)
			return
		}

		etc.mu.Lock()
		defer etc.mu.Unlock()

		for _, tag := range result.Tags {
			etc.tags[*tag.Key] = *tag.Value
		}

		log.Printf("D! loadAllTags: Loaded %d tags for instance %s", len(etc.tags), etc.instanceID)
	})
}

func (etc *EKSTagsCache) getEKSClusterName() string {
	etc.loadAllTags()

	etc.mu.RLock()
	defer etc.mu.RUnlock()

	for tagKey, tagValue := range etc.tags {
		if strings.HasPrefix(tagKey, "kubernetes.io/cluster/") && tagValue == "owned" {
			clusterName := strings.TrimPrefix(tagKey, "kubernetes.io/cluster/")
			if clusterName != "" {
				return clusterName
			}
		}
	}

	if clusterName, exists := etc.tags["eks:cluster-name"]; exists {
		return clusterName
	}

	return ""
}

// For ASG case, the ec2 tag may be not ready as soon as the node is started up.
// In this case, the translator will fail and then the pod will restart.
func GetEKSClusterName(sectionKey string, input map[string]interface{}) string {
	var clusterName string
	if val, ok := input[sectionKey]; ok {
		//The key is in current input instance, use the value in JSON.
		clusterName = val.(string)
	}
	if clusterName == "" {
		clusterName = GetClusterNameFromEc2Tagger()
	}
	return clusterName
}

func GetClusterNameFromEc2Tagger() string {
	instanceID := ec2util.GetEC2UtilSingleton().InstanceID
	if instanceID == "" {
		return ""
	}

	etc := getEKSTagsCache(instanceID)
	return etc.getEKSClusterName()
}
